// Package cvd builds the HTTP handlers cmd/cvd serves.
package cvd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ostendorf-cloud/govue/api"
	"github.com/ostendorf-cloud/govue/health"
	"github.com/ostendorf-cloud/govue/metrics"
	"github.com/ostendorf-cloud/govue/middleware"

	"github.com/ostendorf-cloud/cv/internal/cv"
)

// DefaultLanguage answers a request that names no language.
const DefaultLanguage = "en"

// CodePDFUnavailable is the error code of the PDF route when no PDF was
// loaded for the requested language.
const CodePDFUnavailable = "pdf_unavailable"

// pdfCacheControl lets a polling server reuse a PDF for a minute, then
// revalidate it with If-None-Match, which costs a 304.
const pdfCacheControl = "public, max-age=60"

// rendered is one language's document: the exact bytes cv.RenderJSON or
// cv render pdf wrote for it, and the strong ETag derived from them.
type rendered struct {
	body []byte
	etag string
}

func newRendered(body []byte) rendered {
	sum := sha256.Sum256(body)
	return rendered{body: body, etag: `"` + hex.EncodeToString(sum[:]) + `"`}
}

// documents is every rendered document by language. A map is never changed
// after it is stored: Load, Refresh and LoadPDFs store a new one.
type documents map[string]rendered

// App holds every dependency cmd/cvd wires together.
//
// The data is read once by Load, which cmd/cvd calls before it listens. Until
// Load succeeds, readiness reports unready and the API answers 503. With
// WithGitHub, Refresh adds the GitHub facts of each project to the JSON.
type App struct {
	logger *slog.Logger
	health *health.Registry

	docs atomic.Pointer[documents]
	pdfs atomic.Pointer[documents]

	// mu guards pair and enr, and serialises Load and Refresh. The handlers
	// never take it: they read docs and pdfs.
	mu sync.Mutex
	// pair is the validated data by language, set by Load.
	pair map[string]cv.Document
	// enr is the last good GitHub data by repository.
	enr cv.Enrichment

	// source and ttl are set by WithGitHub. A nil source means no GitHub call.
	source SourceFunc
	ttl    time.Duration

	httpMetrics    *metrics.HTTP
	metricsHandler http.Handler
}

// New constructs an App with no documents loaded.
func New(logger *slog.Logger, opts ...Option) *App {
	registry := metrics.NewRegistry()
	a := &App{
		logger:         logger,
		health:         health.New(),
		httpMetrics:    metrics.NewHTTP(registry),
		metricsHandler: metrics.Handler(registry),
	}
	for _, opt := range opts {
		opt(a)
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
// every language through the JSON renderer the CLI uses, without GitHub
// data, and only then makes the result available. Any problem leaves the App
// unloaded and is returned aggregated: a half document is never served.
// Refresh adds the GitHub data afterwards.
func (a *App) Load(dir string) error {
	de, en, err := cv.LoadPair(filepath.Join(dir, "cv.de.yaml"), filepath.Join(dir, "cv.en.yaml"))
	if err != nil {
		return err
	}

	byLang := map[string]cv.Document{"de": de, "en": en}
	docs, err := renderAll(byLang, nil)
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.pair = byLang
	a.enr = nil
	a.docs.Store(&docs)
	return nil
}

// renderAll renders every language through cv.RenderJSONEnriched, the
// renderer the CLI uses. A nil enr renders what cv.RenderJSON renders.
func renderAll(byLang map[string]cv.Document, enr cv.Enrichment) (documents, error) {
	docs := make(documents, len(cv.Languages))
	for _, lang := range cv.Languages {
		body, err := cv.RenderJSONEnriched(byLang[lang], lang, enr)
		if err != nil {
			return nil, err
		}
		docs[lang] = newRendered(body)
	}
	return docs, nil
}

// pdfMagic starts every PDF file.
var pdfMagic = []byte("%PDF-")

// LoadPDFs reads the PDFs cv render pdf wrote, cv.de.pdf and cv.en.pdf, from
// dir. cvd never runs typst: the image build copies the rendered files in.
// A missing file is not an error. Its language answers 503 on the PDF route,
// and missing names it so the caller can log it. A file that is not a PDF,
// or one that cannot be read, is an error, and no PDF is kept.
func (a *App) LoadPDFs(dir string) (missing []string, err error) {
	pdfs := make(documents, len(cv.Languages))
	for _, lang := range cv.Languages {
		name := cv.PDFFileName(lang)
		body, err := os.ReadFile(filepath.Join(dir, name))
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, name)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("cvd: read %s: %w", name, err)
		}
		if !bytes.HasPrefix(body, pdfMagic) {
			return nil, fmt.Errorf("cvd: %s is not a PDF", name)
		}
		pdfs[lang] = newRendered(body)
	}
	a.pdfs.Store(&pdfs)
	return missing, nil
}

// backticked matches the value excerpt yaml.v3 puts in a type mismatch
// message, for example: cannot unmarshal !!str `Some va...` into ...
var backticked = regexp.MustCompile("`[^`]*`")

// Problems splits an error from Load into one message per problem, safe to
// log. The messages carry positions, ids, evidence refs, unknown YAML keys
// and enumerated values such as a skill level: the data is public by design.
// The free text excerpt the YAML decoder quotes in a type error is replaced,
// best effort: a scalar that itself contains a backtick can get past it.
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
	mux.Handle("/api/v1/cv.pdf", api.Handle(a.getPDF))
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

// errUnknownLanguage is what parseLanguage returns for a well formed lang
// value that names no supported language. Each route decides its status.
var errUnknownLanguage = errors.New("unknown language")

// parseLanguage is the requested language: the single lang query parameter,
// or DefaultLanguage when the request carries none. A malformed query or a
// repeated lang is a 400. An unknown value is errUnknownLanguage. The refused
// value is never echoed back.
func parseLanguage(rawQuery string) (string, error) {
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
	return "", errUnknownLanguage
}

// language is parseLanguage for the JSON route: an unknown language is a 400.
func language(rawQuery string) (string, error) {
	lang, err := parseLanguage(rawQuery)
	if errors.Is(err, errUnknownLanguage) {
		return "", api.Invalid("unknown language").WithDetails(map[string]any{"supported": cv.Languages})
	}
	return lang, err
}

// matches reports whether an If-None-Match header value names etag. The
// comparison is weak, as RFC 9110 requires for this header.
func matches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		// "*" is a wildcard only on its own, never as "W/*".
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

// getCV answers the document in the requested language: the bytes `cv render
// json` writes for it, with a strong ETag. Cache-Control is no-cache, so a
// client revalidates each time, which costs a 304: the data changes on a
// restart and when a refresh brings new GitHub data, but this route exists
// to check the current document, and a max-age would show a stale one.
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

	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	return write(w, r, lang, (*docs)[lang])
}

// getPDF answers the PDF in the requested language: the bytes cv render pdf
// wrote, copied into the image at build time. An unknown language is a 404,
// a language without a loaded PDF a 503 with CodePDFUnavailable. A polling
// server may reuse the answer for a minute and then revalidate with its ETag.
func (a *App) getPDF(w http.ResponseWriter, r *http.Request) error {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.Set("Allow", "GET, HEAD")
		return api.New(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}

	lang, err := parseLanguage(r.URL.RawQuery)
	if errors.Is(err, errUnknownLanguage) {
		return api.NotFound("no PDF in this language").WithDetails(map[string]any{"supported": cv.Languages})
	}
	if err != nil {
		return err
	}

	var doc rendered
	var ok bool
	if pdfs := a.pdfs.Load(); pdfs != nil {
		doc, ok = (*pdfs)[lang]
	}
	if !ok {
		return api.New(http.StatusServiceUnavailable, CodePDFUnavailable, "the cv PDF is not available").Exposed()
	}

	h.Set("Content-Type", "application/pdf")
	h.Set("Content-Disposition", `inline; filename="`+cv.PDFFileName(lang)+`"`)
	h.Set("Cache-Control", pdfCacheControl)
	return write(w, r, lang, doc)
}

// write answers doc with its language and strong ETag, or a 304 when the
// request's If-None-Match names the tag. The caller sets Content-Type and
// Cache-Control.
func write(w http.ResponseWriter, r *http.Request, lang string, doc rendered) error {
	h := w.Header()
	h.Set("Content-Language", lang)
	h.Set("ETag", doc.etag)

	// Repeated header lines are one comma separated list (RFC 9110).
	if match := strings.Join(r.Header.Values("If-None-Match"), ","); match != "" && matches(match, doc.etag) {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}

	h.Set("Content-Length", fmt.Sprint(len(doc.body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return nil
	}
	_, err := w.Write(doc.body)
	return err
}
