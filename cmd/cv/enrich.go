package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/ostendorf-cloud/cv/internal/cv"
	"github.com/ostendorf-cloud/cv/internal/github"
)

// githubFlags are the flags every render subcommand registers to control the
// GitHub enrichment, so all renderers behave alike.
type githubFlags struct {
	off *bool
	url *string
}

// addGithubFlags registers --no-github and --github-url on fs.
func addGithubFlags(fs *flag.FlagSet) githubFlags {
	return githubFlags{
		off: fs.Bool("no-github", false, "do not call GitHub, render from the hand written fields only"),
		url: fs.String("github-url", github.DefaultBaseURL, "base URL of the GitHub REST API"),
	}
}

// enrich returns the enrichment of docs, looked up once for all of them.
// With --no-github it returns nil. Warnings of a degraded lookup go to
// stderr and never fail the render. The only error is a token configuration
// the operator asked for and that cannot be read.
func (g githubFlags) enrich(ctx context.Context, stderr io.Writer, docs ...cv.Document) (cv.Enrichment, error) {
	if *g.off {
		return nil, nil
	}
	token, err := github.TokenFromEnv(os.Getenv, os.ReadFile)
	if err != nil {
		return nil, fmt.Errorf("cv: %w", err)
	}
	client := github.New(github.Config{BaseURL: *g.url, Token: token})
	log := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return cv.Enrich(ctx, client, log, docs...), nil
}
