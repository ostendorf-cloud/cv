package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/ostendorf-cloud/cv/internal/cv"
	"github.com/ostendorf-cloud/cv/internal/github"
)

// githubTimeout is the default budget of all GitHub calls of one render.
const githubTimeout = 60 * time.Second

// githubFlags are the flags every render subcommand registers to control the
// GitHub enrichment, so all renderers behave alike.
type githubFlags struct {
	off     *bool
	url     *string
	timeout *time.Duration
}

// addGithubFlags registers --no-github, --github-url and --github-timeout on fs.
func addGithubFlags(fs *flag.FlagSet) githubFlags {
	return githubFlags{
		off: fs.Bool("no-github", false, "do not call GitHub, render from the hand written fields only"),
		url: fs.String("github-url", github.DefaultBaseURL,
			"base URL of the GitHub REST API; the token is sent to "+github.DefaultBaseURL+" only"),
		timeout: fs.Duration("github-timeout", githubTimeout, "time budget of all GitHub calls of one render"),
	}
}

// tokenFor returns the token to send to baseURL: the token for the real API
// and none for any other host, so --github-url can never carry a credential
// to a host of someone's choosing.
func tokenFor(baseURL, token string) string {
	if baseURL != github.DefaultBaseURL {
		return ""
	}
	return token
}

// enrich returns the enrichment of docs, looked up once for all of them.
// With --no-github it returns nil. Warnings of a degraded lookup go to
// stderr and never fail the render. The calls share one overall deadline.
// The only error is a token configuration the operator asked for and that
// cannot be read.
func (g githubFlags) enrich(ctx context.Context, stderr io.Writer, docs ...cv.Document) (cv.Enrichment, error) {
	if *g.off {
		return nil, nil
	}
	token, err := github.TokenFromEnv(os.Getenv, os.ReadFile)
	if err != nil {
		return nil, fmt.Errorf("cv: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, *g.timeout)
	defer cancel()
	client := github.New(github.Config{BaseURL: *g.url, Token: tokenFor(*g.url, token)})
	log := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return cv.Enrich(ctx, client, log, docs...), nil
}
