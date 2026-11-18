// Package cvd builds the HTTP handlers cmd/cvd serves.
package cvd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/ostendorf-cloud/govue/api"
	"github.com/ostendorf-cloud/govue/health"
	"github.com/ostendorf-cloud/govue/metrics"
	"github.com/ostendorf-cloud/govue/middleware"

	"github.com/ostendorf-cloud/cv/internal/cv"
)

// DefaultLanguage answers a request that names no language.
const DefaultLanguage = "en"

// rendered is one language's document: the exact bytes cv.RenderJSON wrote
// for it, computed once, and the strong ETag derived from them.
type rendered struct {
	body []byte
	etag string
}

// documents is every rendered document by language. It is built once by
// Load and never changed afterwards.
type documents map[string]rendered

// App holds every dependency cmd/cvd wires together.
//
// The documents it serves are read once by Load, which cmd/cvd calls before
// it listens. Until Load succeeds, readiness reports unready and the API
// answers 503.
type App struct {
	logger *slog.Logger
	health *health.Registry

	docs atomic.Pointer[documents]

	httpMetrics    *metrics.HTTP
	metricsHandler http.Handler
}

// New constructs an App with no documents loaded.
func New(logger *slog.Logger) *App {
	registry := metrics.NewRegistry()
	a := &App{
		logger:         logger,
		health:         health.New(),
		httpMetrics:    metrics.NewHTTP(registry),
		metricsHandler: metrics.Handler(registry),
	}
	a.health.Add("cv data", 0, func(context.Context) error {
		if a.docs.Load() == nil {
			return errors.New("cv documents are not loaded")
		}
		return nil
	})
	return a
}

// Load reads cv.de.yaml and cv.en.yaml from dir, validates them, renders
// every language through cv.RenderJSON, the renderer the CLI uses, and only
// then makes the result available. Any problem leaves the App unloaded and
// is returned aggregated: a half document is never served.
func (a *App) Load(dir string) error {
	de, en, err := cv.LoadPair(filepath.Join(dir, "cv.de.yaml"), filepath.Join(dir, "cv.en.yaml"))
	if err != nil {
		return err
	}

	byLang := map[string]cv.Document{"de": de, "en": en}
	docs := make(documents, len(cv.Languages))
	for _, lang := range cv.Languages {
		body, err := cv.RenderJSON(byLang[lang], lang)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		docs[lang] = rendered{body: body, etag: `"` + hex.EncodeToString(sum[:]) + `"`}
	}
	a.docs.Store(&docs)
	return nil
}

// backticked matches the value excerpt yaml.v3 puts in a type mismatch
// message, for example: cannot unmarshal !!str `Some va...` into ...
var backticked = regexp.MustCompile("`[^`]*`")

// Problems splits an error from Load into one message per problem, safe to
// log: a value excerpt the YAML decoder quotes is replaced. Validation
// messages name sections, positions and ids; they carry no field value
// beyond an enumerated one such as a skill level.
func Problems(err error) []string {
	var joined interface{ Unwrap() []error }
	if !errors.As(err, &joined) {
		return []string{backticked.ReplaceAllString(err.Error(), "`<redacted>`")}
	}
	var out []string
	for _, e := range joined.Unwrap() {
		out = append(out, Problems(e)...)
	}
	return out
}

// Health is the readiness registry httpserver's BeforeShutdown drains.
func (a *App) Health() *health.Registry {
	return a.health
}

// Handler is the public handler cmd/cvd serves on the primary listener.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	// No method in the pattern: the mux's own 405 is plain text, and the
	// error shape belongs to govue's api package.
	mux.Handle("/api/v1/cv", api.Handle(a.getCV))
	mux.Handle("/", api.NotFoundHandler())

	return middleware.Chain(mux,
		middleware.Recover(a.logger),
		middleware.RequestID(a.logger),
		a.httpMetrics.Middleware,
	)
}

// AdminHandler is what httpserver.RunAdmin serves on the second,
// administrative listener: liveness, readiness and the Prometheus scrape,
// none of which a caller of the public handler ever reaches.
func (a *App) AdminHandler() http.Handler {
	mux := http.NewServeMux()
	a.health.Handle(mux)
	mux.Handle("GET /metrics", a.metricsHandler)

	return middleware.Chain(mux,
		middleware.Recover(a.logger),
		middleware.RequestID(a.logger),
	)
}

// language is the requested language: the single lang query parameter, or
// DefaultLanguage when the request carries none. Anything else is refused,
// and the refused value is never echoed back.
func language(rawQuery string) (string, error) {
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", api.Invalid("the query string is malformed")
	}
	values, present := query["lang"]
	if !present {
		return DefaultLanguage, nil
	}
	if len(values) != 1 {
		return "", api.Invalid("lang must be given once")
	}
	for _, known := range cv.Languages {
		if values[0] == known {
			return known, nil
		}
	}
	return "", api.Invalid("unknown language").WithDetails(map[string]any{"supported": cv.Languages})
}

// matches reports whether an If-None-Match header value names etag. The
// comparison is weak, as RFC 9110 requires for this header.
func matches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}

// getCV answers the document in the requested language: the bytes `cv render
// json` writes for it, with a strong ETag. Cache-Control is no-cache, so a
// client revalidates each time, which costs a 304: the data changes only on
// a restart, but this route exists to check the current document, and a
// max-age would show a stale one after a restart.
func (a *App) getCV(w http.ResponseWriter, r *http.Request) error {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.Set("Allow", "GET, HEAD")
		return api.New(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}

	docs := a.docs.Load()
	if docs == nil {
		return api.Unavailable("the cv document is not loaded").Exposed()
	}

	lang, err := language(r.URL.RawQuery)
	if err != nil {
		return err
	}
	doc := (*docs)[lang]

	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Content-Language", lang)
	h.Set("Cache-Control", "no-cache")
	h.Set("ETag", doc.etag)

	if match := r.Header.Get("If-None-Match"); match != "" && matches(match, doc.etag) {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}

	h.Set("Content-Length", fmt.Sprint(len(doc.body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return nil
	}
	_, err = w.Write(doc.body)
	return err
}
