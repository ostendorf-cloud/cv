package cvd

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/cv/internal/cv"
)

const (
	validDir   = "../../cmd/cv/testdata/render-pair"
	invalidDir = "../cv/testdata/invalid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// loaded is an App with the valid fixture pair loaded.
func loaded(t *testing.T) *App {
	t.Helper()
	app := New(discardLogger())
	if err := app.Load(validDir); err != nil {
		t.Fatalf("Load(%s) error = %v", validDir, err)
	}
	return app
}

// copyAsPair writes one fixture file as both language files of a new
// directory, so a single invalid fixture exercises the pair loader.
func copyAsPair(t *testing.T, fixture string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(invalidDir, fixture))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"cv.de.yaml", "cv.en.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func get(t *testing.T, app *App, method, target string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	return rec
}

// expected is what RenderJSON writes for the fixture pair, per language.
func expected(t *testing.T) map[string][]byte {
	t.Helper()
	de, en, err := cv.LoadPair(filepath.Join(validDir, "cv.de.yaml"), filepath.Join(validDir, "cv.en.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for lang, doc := range map[string]cv.Document{"de": de, "en": en} {
		body, err := cv.RenderJSON(doc, lang)
		if err != nil {
			t.Fatal(err)
		}
		out[lang] = body
	}
	return out
}

func TestDocumentEqualsRenderJSON(t *testing.T) {
	t.Parallel()

	want := expected(t)
	app := loaded(t)

	tests := []struct {
		name   string
		target string
		lang   string
	}{
		{"german", "/api/v1/cv?lang=de", "de"},
		{"english", "/api/v1/cv?lang=en", "en"},
		{"default language", "/api/v1/cv", "en"},
		{"other query parameters are ignored", "/api/v1/cv?x=1&lang=de", "de"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := get(t, app, http.MethodGet, tt.target, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body %q", rec.Code, rec.Body.String())
			}
			if got := rec.Body.Bytes(); string(got) != string(want[tt.lang]) {
				t.Errorf("body differs from RenderJSON for %s:\n%s", tt.lang, got)
			}
			var doc struct {
				SchemaVersion int    `json:"schema_version"`
				Language      string `json:"language"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.SchemaVersion != cv.SchemaVersion {
				t.Errorf("schema_version = %d, want %d", doc.SchemaVersion, cv.SchemaVersion)
			}
			if doc.Language != tt.lang {
				t.Errorf("language = %q, want %q", doc.Language, tt.lang)
			}
		})
	}
}

func TestDefaultLanguageIsDocumented(t *testing.T) {
	t.Parallel()

	if DefaultLanguage != "en" {
		t.Errorf("DefaultLanguage = %q, README documents en", DefaultLanguage)
	}
}

func TestHeaders(t *testing.T) {
	t.Parallel()

	app := loaded(t)
	rec := get(t, app, http.MethodGet, "/api/v1/cv?lang=de", nil)

	for header, want := range map[string]string{
		"Content-Type":           "application/json; charset=utf-8",
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-cache",
		"Content-Language":       "de",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	etag := rec.Header().Get("ETag")
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) || strings.HasPrefix(etag, "W/") {
		t.Errorf("ETag = %q, want a strong quoted tag", etag)
	}
}

func TestETag(t *testing.T) {
	t.Parallel()

	app := loaded(t)
	etagOf := func(target string) string {
		return get(t, app, http.MethodGet, target, nil).Header().Get("ETag")
	}
	de, en := etagOf("/api/v1/cv?lang=de"), etagOf("/api/v1/cv?lang=en")

	if de == en {
		t.Error("the two languages share one ETag")
	}
	if again := etagOf("/api/v1/cv?lang=de"); again != de {
		t.Errorf("ETag changed between requests: %q then %q", de, again)
	}
	other := loaded(t)
	if got := get(t, other, http.MethodGet, "/api/v1/cv?lang=de", nil).Header().Get("ETag"); got != de {
		t.Errorf("ETag differs between two loads of the same data: %q and %q", de, got)
	}

	tests := []struct {
		name        string
		ifNoneMatch string
		want        int
	}{
		{"matching tag", de, http.StatusNotModified},
		{"weak form of the tag", "W/" + de, http.StatusNotModified},
		{"one of a list", `"other", ` + de, http.StatusNotModified},
		{"wildcard", "*", http.StatusNotModified},
		{"the other language's tag", en, http.StatusOK},
		{"unknown tag", `"nope"`, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := get(t, app, http.MethodGet, "/api/v1/cv?lang=de", http.Header{"If-None-Match": {tt.ifNoneMatch}})
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusNotModified {
				if rec.Body.Len() != 0 {
					t.Errorf("304 carries a body of %d bytes", rec.Body.Len())
				}
				if rec.Header().Get("ETag") != de {
					t.Errorf("304 ETag = %q, want %q", rec.Header().Get("ETag"), de)
				}
			}
		})
	}
}

func TestHead(t *testing.T) {
	t.Parallel()

	want := expected(t)["en"]
	rec := get(t, loaded(t), http.MethodHead, "/api/v1/cv?lang=en", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("HEAD carries a body of %d bytes", rec.Body.Len())
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(want)) {
		t.Errorf("Content-Length = %q, want %d", got, len(want))
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("HEAD carries no ETag")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	t.Parallel()

	app := loaded(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			rec := get(t, app, method, "/api/v1/cv", nil)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
				t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
			}
			assertErrorShape(t, rec, "method_not_allowed")
		})
	}
}

func assertErrorShape(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not the error shape: %v", rec.Body.String(), err)
	}
	if body.Error.Code != code {
		t.Errorf("error code = %q, want %q", body.Error.Code, code)
	}
	if body.Error.Message == "" {
		t.Error("error message is empty")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q on an error, want nosniff", got)
	}
}

func TestUnknownLanguageIsRefused(t *testing.T) {
	t.Parallel()

	app := loaded(t)
	tests := []struct {
		name  string
		query string
	}{
		{"empty", "lang="},
		{"unknown code", "lang=fr"},
		{"wrong case", "lang=EN"},
		{"padded", "lang=%20en"},
		{"injection looking", "lang=" + url.QueryEscape(`en"><script>alert(1)</script>`)},
		{"path traversal", "lang=" + url.QueryEscape("../../etc/passwd")},
		{"very long", "lang=" + strings.Repeat("a", 10000)},
		{"given twice", "lang=de&lang=en"},
		{"malformed escape", "lang=%zz"},
		{"a locale tag", "lang=en-US"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := get(t, app, http.MethodGet, "/api/v1/cv?"+tt.query, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			assertErrorShape(t, rec, "invalid_argument")
			if len(rec.Body.String()) > 500 {
				t.Errorf("error body is %d bytes: the input is echoed", rec.Body.Len())
			}
			if strings.Contains(rec.Body.String(), "script") || strings.Contains(rec.Body.String(), "passwd") {
				t.Errorf("error body echoes the input: %s", rec.Body.String())
			}
		})
	}
}

func TestAcceptLanguageIsNotUsed(t *testing.T) {
	t.Parallel()

	want := expected(t)["en"]
	rec := get(t, loaded(t), http.MethodGet, "/api/v1/cv", http.Header{"Accept-Language": {"de"}})
	if rec.Body.String() != string(want) {
		t.Error("Accept-Language changed the document, the language comes from lang only")
	}
	if rec.Header().Get("Vary") != "" {
		t.Errorf("Vary = %q, want none", rec.Header().Get("Vary"))
	}
}

func TestUnloadedAppAnswers503(t *testing.T) {
	t.Parallel()

	rec := get(t, New(discardLogger()), http.MethodGet, "/api/v1/cv", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	assertErrorShape(t, rec, "unavailable")
}

func TestLoadRefusesInvalidData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dir     func(t *testing.T) string
		wantIn  string
		wantNot []string
	}{
		{"duplicate id", func(t *testing.T) string { return copyAsPair(t, "duplicate-id.yaml") }, "duplicate id", nil},
		{
			"missing required field", func(t *testing.T) string { return copyAsPair(t, "missing-required-field.yaml") }, "is required",
			[]string{"Nebulon", "contact@example.com", "2022-03"},
		},
		{"malformed yaml", func(t *testing.T) string { return copyAsPair(t, "malformed-yaml.yaml") }, "", nil},
		{"unknown field", func(t *testing.T) string { return copyAsPair(t, "unknown-field.yaml") }, "", nil},
		{"directory without data", func(t *testing.T) string { return t.TempDir() }, "no such file", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := New(discardLogger())
			err := app.Load(tt.dir(t))
			if err == nil {
				t.Fatal("Load() error = nil, want a refusal")
			}
			if app.docs.Load() != nil {
				t.Error("documents are available after a failed Load")
			}
			if rec := get(t, app, http.MethodGet, "/api/v1/cv", nil); rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status after a failed Load = %d, want 503", rec.Code)
			}
			if tt.wantIn != "" && !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error %q lacks %q", err, tt.wantIn)
			}
			joined := strings.Join(Problems(err), "\n")
			for _, value := range tt.wantNot {
				if strings.Contains(joined, value) {
					t.Errorf("a problem message echoes the value %q:\n%s", value, joined)
				}
			}
		})
	}
}

func TestLoadAggregatesProblems(t *testing.T) {
	t.Parallel()

	err := New(discardLogger()).Load(copyAsPair(t, "multiple-problems.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil")
	}
	if n := len(Problems(err)); n < 4 {
		t.Errorf("Problems() = %d messages, want the problems of both files together:\n%v", n, err)
	}
}

func TestProblemsRedactsYAMLValueExcerpts(t *testing.T) {
	t.Parallel()

	err := errors.New("yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `Fake Va...` into []cv.ExperienceEntry")
	got := strings.Join(Problems(err), "\n")
	if strings.Contains(got, "Fake") {
		t.Errorf("excerpt not redacted: %s", got)
	}
	if !strings.Contains(got, "cannot unmarshal !!str") {
		t.Errorf("message lost its meaning: %s", got)
	}
}

func TestReadiness(t *testing.T) {
	t.Parallel()

	app := New(discardLogger())
	ready := func() int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		app.AdminHandler().ServeHTTP(rec, req)
		return rec.Code
	}
	live := func() int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		app.AdminHandler().ServeHTTP(rec, req)
		return rec.Code
	}

	if got := ready(); got != http.StatusServiceUnavailable {
		t.Errorf("readiness before Load = %d, want 503", got)
	}
	if got := live(); got != http.StatusOK {
		t.Errorf("liveness before Load = %d, want 200", got)
	}

	if err := app.Load(copyAsPair(t, "duplicate-id.yaml")); err == nil {
		t.Fatal("Load() accepted invalid data")
	}
	if got := ready(); got != http.StatusServiceUnavailable {
		t.Errorf("readiness after a failed Load = %d, want 503", got)
	}

	if err := app.Load(validDir); err != nil {
		t.Fatal(err)
	}
	if got := ready(); got != http.StatusOK {
		t.Errorf("readiness after Load = %d, want 200", got)
	}
	if got := live(); got != http.StatusOK {
		t.Errorf("liveness after Load = %d, want 200", got)
	}
}

func TestHandlerAnswersAnUnknownRouteInTheErrorShape(t *testing.T) {
	t.Parallel()

	rec := get(t, loaded(t), http.MethodGet, "/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminHandlerAnswersMetrics(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	loaded(t).AdminHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("/metrics status = %d, want 200", rec.Code)
	}
}
