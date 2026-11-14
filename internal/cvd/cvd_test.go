package cvd

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestHandlerAnswersThePlaceholderDocument(t *testing.T) {
	t.Parallel()

	app := New(discardLogger())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/cv", nil)
	rec := httptest.NewRecorder()

	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "{}" {
		t.Errorf("body = %q, want %q", got, "{}")
	}
}

func TestHandlerAnswersAnUnknownRouteInTheErrorShape(t *testing.T) {
	t.Parallel()

	app := New(discardLogger())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()

	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminHandlerAnswersLivenessReadinessAndMetrics(t *testing.T) {
	t.Parallel()

	app := New(discardLogger())

	for _, tt := range []struct {
		path string
		want int
	}{
		{"/healthz", http.StatusOK},
		{"/readyz", http.StatusOK},
		{"/metrics", http.StatusOK},
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil)
		rec := httptest.NewRecorder()

		app.AdminHandler().ServeHTTP(rec, req)

		if rec.Code != tt.want {
			t.Errorf("%s status = %d, want %d", tt.path, rec.Code, tt.want)
		}
	}
}
