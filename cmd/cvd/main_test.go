package main

import (
	"log/slog"
	"testing"

	"github.com/ostendorf-cloud/cv/internal/cvd"
	"github.com/ostendorf-cloud/cv/internal/cvdconfig"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// TestConfigBuildsAWorkingApp proves cvdconfig.Load's result is exactly what
// cvd.New expects, without starting a server or binding a port: a change to
// either package that breaks the other fails here instead of at a real
// deployment.
func TestConfigBuildsAWorkingApp(t *testing.T) {
	cfg, err := cvdconfig.Load(func(string) string { return "" })
	if err != nil {
		t.Fatalf("cvdconfig.Load() error = %v", err)
	}
	if cfg.Addr == "" {
		t.Error("cfg.Addr is empty, want the documented default")
	}

	app := cvd.New(discardLogger())
	if app.Handler() == nil {
		t.Error("app.Handler() = nil, want a working handler")
	}
	if app.AdminHandler() == nil {
		t.Error("app.AdminHandler() = nil, want a working handler")
	}
	if app.Health() == nil {
		t.Error("app.Health() = nil, want a working health registry")
	}
}
