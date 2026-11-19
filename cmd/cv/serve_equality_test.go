package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ostendorf-cloud/cv/internal/cv"
	"github.com/ostendorf-cloud/cv/internal/cvd"
)

// TestServedDocumentEqualsTheArtefact proves cvd answers, per language, the
// same bytes `cv render json` writes: the files the CLI code path produces
// are compared with what the handler serves from the same data.
func TestServedDocumentEqualsTheArtefact(t *testing.T) {
	t.Parallel()

	out := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"render", "json", "--dir", "testdata/render-pair", "--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}

	app := cvd.New(slog.New(slog.DiscardHandler))
	if err := app.Load("testdata/render-pair"); err != nil {
		t.Fatal(err)
	}

	for _, lang := range cv.Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			artefact, err := os.ReadFile(filepath.Join(out, cv.JSONFileName(lang)))
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/cv?lang="+lang, nil)
			rec := httptest.NewRecorder()
			app.Handler().ServeHTTP(rec, req)

			got, err := io.ReadAll(rec.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, artefact) {
				t.Errorf("served %s differs from the artefact the CLI wrote", lang)
			}
		})
	}
}
