package cvd

import (
	"context"
	"maps"
	"time"

	"github.com/ostendorf-cloud/cv/internal/cv"
)

// refreshTimeout bounds all GitHub calls of one refresh, the same budget one
// `cv render json` has.
const refreshTimeout = time.Minute

// SourceFunc returns the source one refresh asks GitHub through. Refresh calls
// it once per cycle, so a source's own cache and call budget never outlive
// the cycle: *github.Client keeps a result for its whole life.
type SourceFunc func() cv.RepoSource

// Option configures an App.
type Option func(*App)

// WithGitHub makes Refresh look up the languages and latest release of each
// project through source, and RefreshEvery repeat that every ttl. Without it
// the App makes no GitHub call and serves the documents Load rendered.
func WithGitHub(source SourceFunc, ttl time.Duration) Option {
	return func(a *App) {
		a.source = source
		a.ttl = ttl
	}
}

// Refresh looks up the GitHub data of every project once and renders both
// documents again with it. A repository whose lookup failed keeps its last
// good data, and one never looked up successfully renders without the
// `languages` and `latest_release` keys. A GitHub failure never fails the
// API: Refresh only logs, through cv.Enrich, and the served documents stay as
// they were. It does nothing before Load succeeded or without WithGitHub.
func (a *App) Refresh(ctx context.Context) {
	if a.source == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pair == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()
	fresh := cv.Enrich(ctx, a.source(), a.logger, a.pair["de"], a.pair["en"])

	merged := make(cv.Enrichment, len(a.enr)+len(fresh))
	maps.Copy(merged, a.enr)
	maps.Copy(merged, fresh)

	docs, err := renderAll(a.pair, merged)
	if err != nil {
		a.logger.ErrorContext(ctx, "rendering the github data failed, serving the previous documents", "error", err)
		return
	}
	a.enr = merged
	a.docs.Store(&docs)
	a.logger.InfoContext(ctx, "github data refreshed", "fresh", len(fresh), "kept", len(merged)-len(fresh))
}

// RefreshEvery calls Refresh every ttl until ctx is done. The first call is
// the caller's: cmd/cvd calls Refresh before it listens. It returns at once
// without WithGitHub or with a ttl that is not positive.
func (a *App) RefreshEvery(ctx context.Context) {
	if a.source == nil || a.ttl <= 0 {
		return
	}
	ticker := time.NewTicker(a.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.Refresh(ctx)
		}
	}
}
