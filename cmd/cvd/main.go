// Command cvd serves the data cmd/cv renders from, over HTTP, reached only
// from inside the cluster. It answers GET /api/v1/cv on its public listener
// and health, readiness and metrics on govue's admin listener. See
// AGENTS.md's "Scope of this milestone" for what it does not do yet.
package main

import (
	"context"
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
