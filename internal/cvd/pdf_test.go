package cvd

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// fixturePDF is what a test writes for lang: not a real PDF, only the header
// LoadPDFs checks and a body that differs per language.
func fixturePDF(lang string) []byte {
	return []byte("%PDF-1.7\n% fixture " + lang + "\n%%EOF\n")
}

// pdfDir writes the fixture PDF of each given language into a new directory.
func pdfDir(t *testing.T, langs ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, lang := range langs {
		if err := os.WriteFile(filepath.Join(dir, "cv."+lang+".pdf"), fixturePDF(lang), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// withPDFs is a loaded App with the fixture PDFs of langs.
func withPDFs(t *testing.T, langs ...string) *App {
	t.Helper()
	app := loaded(t)
	if _, err := app.LoadPDFs(pdfDir(t, langs...)); err != nil {
		t.Fatalf("LoadPDFs() error = %v", err)
	}
	return app
}

func TestPDFServesTheRenderedBytes(t *testing.T) {
	t.Parallel()

	app := withPDFs(t, "de", "en")
	tests := []struct {
		target string
		lang   string
	}{
		{"/api/v1/cv.pdf?lang=de", "de"},
		{"/api/v1/cv.pdf?lang=en", "en"},
		{"/api/v1/cv.pdf", "en"},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			t.Parallel()

			rec := get(t, app, http.MethodGet, tt.target, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body %q", rec.Code, rec.Body.String())
			}
			want := fixturePDF(tt.lang)
			if got := rec.Body.Bytes(); string(got) != string(want) {
				t.Errorf("body = %q, want %q", got, want)
			}
			sum := sha256.Sum256(want)
			for header, value := range map[string]string{
				"Content-Type":           "application/pdf",
				"Content-Disposition":    `inline; filename="cv.` + tt.lang + `.pdf"`,
				"Content-Language":       tt.lang,
				"Cache-Control":          "public, max-age=60",
				"ETag":                   `"` + hex.EncodeToString(sum[:]) + `"`,
				"Content-Length":         strconv.Itoa(len(want)),
				"X-Content-Type-Options": "nosniff",
			} {
				if got := rec.Header().Get(header); got != value {
					t.Errorf("%s = %q, want %q", header, got, value)
				}
			}
		})
	}
}

func TestPDFConditionalRequest(t *testing.T) {
	t.Parallel()

	app := withPDFs(t, "de", "en")
	etag := get(t, app, http.MethodGet, "/api/v1/cv.pdf?lang=de", nil).Header().Get("ETag")
	other := get(t, app, http.MethodGet, "/api/v1/cv.pdf?lang=en", nil).Header().Get("ETag")
	if etag == other {
		t.Fatal("the two languages share one ETag")
	}

	rec := get(t, app, http.MethodGet, "/api/v1/cv.pdf?lang=de", http.Header{"If-None-Match": {etag}})
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("matching tag: status = %d with %d bytes, want 304 and no body", rec.Code, rec.Body.Len())
	}
	if rec.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Errorf("304 Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}

	rec = get(t, app, http.MethodGet, "/api/v1/cv.pdf?lang=de", http.Header{"If-None-Match": {other}})
	if rec.Code != http.StatusOK {
		t.Errorf("other tag: status = %d, want 200", rec.Code)
	}
}

func TestPDFHead(t *testing.T) {
	t.Parallel()

	rec := get(t, withPDFs(t, "de", "en"), http.MethodHead, "/api/v1/cv.pdf?lang=de", nil)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("status = %d with %d bytes, want 200 and no body", rec.Code, rec.Body.Len())
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(fixturePDF("de"))) {
		t.Errorf("Content-Length = %q", got)
	}
}

func TestPDFRefusals(t *testing.T) {
	t.Parallel()

	app := withPDFs(t, "de", "en")
	tests := []struct {
		name   string
		method string
		query  string
		status int
		code   string
	}{
		{"unknown language", http.MethodGet, "lang=fr", http.StatusNotFound, "not_found"},
		{"empty language", http.MethodGet, "lang=", http.StatusNotFound, "not_found"},
		{"injection looking", http.MethodGet, "lang=%3Cscript%3E", http.StatusNotFound, "not_found"},
		{"given twice", http.MethodGet, "lang=de&lang=en", http.StatusBadRequest, "invalid_argument"},
		{"malformed escape", http.MethodGet, "lang=%zz", http.StatusBadRequest, "invalid_argument"},
		{"post", http.MethodPost, "lang=de", http.StatusMethodNotAllowed, "method_not_allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := get(t, app, tt.method, "/api/v1/cv.pdf?"+tt.query, nil)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			assertErrorShape(t, rec, tt.code)
			if strings.Contains(rec.Body.String(), "script") {
				t.Errorf("error body echoes the input: %s", rec.Body.String())
			}
		})
	}
}

func TestPDFUnavailable(t *testing.T) {
	t.Parallel()

	t.Run("no PDF directory loaded", func(t *testing.T) {
		t.Parallel()

		app := loaded(t)
		rec := get(t, app, http.MethodGet, "/api/v1/cv.pdf?lang=de", nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		assertErrorShape(t, rec, CodePDFUnavailable)
		// The JSON route is unaffected.
		if rec := get(t, app, http.MethodGet, "/api/v1/cv?lang=de", nil); rec.Code != http.StatusOK {
			t.Errorf("JSON status without PDFs = %d, want 200", rec.Code)
		}
	})

	t.Run("one language missing", func(t *testing.T) {
		t.Parallel()

		app := loaded(t)
		missing, err := app.LoadPDFs(pdfDir(t, "de"))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(missing, []string{"cv.en.pdf"}) {
			t.Errorf("missing = %v, want [cv.en.pdf]", missing)
		}
		if rec := get(t, app, http.MethodGet, "/api/v1/cv.pdf?lang=de", nil); rec.Code != http.StatusOK {
			t.Errorf("de status = %d, want 200", rec.Code)
		}
		rec := get(t, app, http.MethodGet, "/api/v1/cv.pdf?lang=en", nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("en status = %d, want 503", rec.Code)
		}
		assertErrorShape(t, rec, CodePDFUnavailable)
	})

	t.Run("an unknown language is a 404 even without PDFs", func(t *testing.T) {
		t.Parallel()

		if rec := get(t, loaded(t), http.MethodGet, "/api/v1/cv.pdf?lang=fr", nil); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestLoadPDFsRefusesAFileThatIsNotAPDF(t *testing.T) {
	t.Parallel()

	dir := pdfDir(t, "en")
	if err := os.WriteFile(filepath.Join(dir, "cv.de.pdf"), []byte("<html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := loaded(t)
	if _, err := app.LoadPDFs(dir); err == nil || !strings.Contains(err.Error(), "cv.de.pdf is not a PDF") {
		t.Fatalf("LoadPDFs() error = %v, want one naming cv.de.pdf", err)
	}
	if app.pdfs.Load() != nil {
		t.Error("PDFs are kept after a failed LoadPDFs")
	}
	if rec := get(t, app, http.MethodGet, "/api/v1/cv.pdf?lang=en", nil); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
