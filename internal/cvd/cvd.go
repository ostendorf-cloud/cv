// Package cvd builds the HTTP handlers cmd/cvd serves.
package cvd

import (
	"log/slog"
	"net/http"

	"github.com/ostendorf-cloud/govue/api"
	"github.com/ostendorf-cloud/govue/health"
	"github.com/ostendorf-cloud/govue/metrics"
	"github.com/ostendorf-cloud/govue/middleware"
)

// App holds every dependency cmd/cvd wires together.
//
// The data getCV answers is a placeholder: data/cv.yaml and internal/cv, the
// validated schema it will eventually be read through, are later
// milestones. See AGENTS.md's "Scope of this milestone".
type App struct {
	logger *slog.Logger
	health *health.Registry

	httpMetrics    *metrics.HTTP
	metricsHandler http.Handler
}

// New constructs an App.
func New(logger *slog.Logger) *App {
	registry := metrics.NewRegistry()
	return &App{
		logger:         logger,
		health:         health.New(),
		httpMetrics:    metrics.NewHTTP(registry),
		metricsHandler: metrics.Handler(registry),
	}
}

// Health is the readiness registry httpserver's BeforeShutdown drains.
func (a *App) Health() *health.Registry {
	return a.health
}

// Handler is the public handler cmd/cvd serves on the primary listener.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/cv", api.Handle(a.getCV))
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

// document is the one shape GET /api/v1/cv answers today. Empty until
// data/cv.yaml and internal/cv exist.
type document struct{}

// getCV answers the placeholder CV document.
func (a *App) getCV(w http.ResponseWriter, _ *http.Request) error {
	return api.Write(w, http.StatusOK, document{})
}
