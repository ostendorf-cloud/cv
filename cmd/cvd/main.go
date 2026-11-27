// Command cvd serves the data cmd/cv renders from, over HTTP, reached only
// from inside the cluster. It answers GET /api/v1/cv and GET /api/v1/cv.pdf on
// its public listener and health, readiness and metrics on govue's admin
// listener. See
// AGENTS.md's "Scope of this milestone" for what it does not do yet.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ostendorf-cloud/govue/httpserver"
	"github.com/ostendorf-cloud/govue/logging"

	"github.com/ostendorf-cloud/cv/internal/cvd"
	"github.com/ostendorf-cloud/cv/internal/cvdconfig"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := cvdconfig.Load(os.Getenv)
	if err != nil {
		return err
	}

	logger := logging.New(os.Stdout, cfg.LogLevel)

	app := cvd.New(logger)

	// Read once, before anything listens: invalid data stops the start, so
	// a half document is never served. The problems are logged one by one,
	// with positions, ids and enumerated values, and free text excerpts
	// redacted on a best effort basis; see cvd.Problems.
	if err := app.Load(cfg.DataDir); err != nil {
		for _, problem := range cvd.Problems(err) {
			logger.Error("cv data is invalid", "problem", problem)
		}
		return errors.New("cvd: refusing to start: the cv data is invalid")
	}

	// The PDFs are rendered at build time, never here. Without them only the
	// PDF route answers 503, so a local run without typst still works.
	if cfg.PDFDir == "" {
		logger.Info("PDF_DIR is not set, the PDF route answers 503")
	} else {
		missing, err := app.LoadPDFs(cfg.PDFDir)
		if err != nil {
			return fmt.Errorf("cvd: refusing to start: %w", err)
		}
		for _, name := range missing {
			logger.Warn("a PDF is missing, its language answers 503", "file", name, "dir", cfg.PDFDir)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return httpserver.RunAdmin(ctx, httpserver.Config{
		Addr:    cfg.Addr,
		Handler: app.Handler(),
		Logger:  logger,
		// Failing the readiness probe before the drain delay starts is what
		// lets a load balancer stop sending new requests before the server
		// stops accepting connections at all.
		BeforeShutdown: app.Health().Drain,
	}, httpserver.AdminConfig{
		Addr:    cfg.AdminAddr,
		Handler: app.AdminHandler(),
	})
}
