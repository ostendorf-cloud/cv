package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/cv/internal/cvd"
	"github.com/ostendorf-cloud/cv/internal/cvdconfig"
)

const validDir = "../cv/testdata/render-pair"

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// TestConfigBuildsAWorkingApp proves cvdconfig.Load's result is exactly what
// cvd.New and App.Load expect, without starting a server or binding a port: a
// change to either package that breaks the other fails here instead of at a
// real deployment.
func TestConfigBuildsAWorkingApp(t *testing.T) {
	cfg, err := cvdconfig.Load(func(key string) string {
		if key == "DATA_DIR" {
			return validDir
		}
		return ""
	})
	if err != nil {
		t.Fatalf("cvdconfig.Load() error = %v", err)
	}
	if cfg.Addr == "" {
		t.Error("cfg.Addr is empty, want the documented default")
	}

	app := cvd.New(discardLogger())
	if err := app.Load(cfg.DataDir); err != nil {
		t.Fatalf("app.Load() error = %v", err)
	}
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

// TestRunRefusesToStartOnInvalidData runs the real startup path against a
// directory holding a fixture with a duplicate id: run must return before it
// listens, so the test cannot hang on a server.
func TestRunRefusesToStartOnInvalidData(t *testing.T) {
	data, err := os.ReadFile("../../internal/cv/testdata/invalid/duplicate-id.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"cv.de.yaml", "cv.en.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DATA_DIR", dir)

	err = run()
	if err == nil || !strings.Contains(err.Error(), "refusing to start") {
		t.Fatalf("run() error = %v, want a refusal to start", err)
	}
}

// TestRunRefusesToStartOnAFileThatIsNotAPDF runs the real startup path with
// valid data and a PDF directory holding a file that is not a PDF: run must
// return before it listens.
func TestRunRefusesToStartOnAFileThatIsNotAPDF(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cv.de.pdf"), []byte("<html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", validDir)
	t.Setenv("PDF_DIR", dir)

	err := run()
	if err == nil || !strings.Contains(err.Error(), "refusing to start") || !strings.Contains(err.Error(), "not a PDF") {
		t.Fatalf("run() error = %v, want a refusal naming the file that is not a PDF", err)
	}
}

func TestRunRefusesToStartWithoutADataDir(t *testing.T) {
	t.Setenv("DATA_DIR", "")

	if err := run(); err == nil || !strings.Contains(err.Error(), "DATA_DIR") {
		t.Fatalf("run() error = %v, want a DATA_DIR problem", err)
	}
}
