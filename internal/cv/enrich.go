package cv

import (
	"context"
	"log/slog"

	"github.com/ostendorf-cloud/cv/internal/github"
)

// RepoSource looks up what GitHub knows about one repository, spelled
// owner/name. *github.Client satisfies it.
type RepoSource interface {
	Repo(ctx context.Context, repo string) (github.Info, error)
}

// Enrichment is the derived view of a render: what GitHub knows about each
// project's repository, keyed by the project's github_repo. It is computed at
// render time, handed to every renderer and never written to a data file, and
// it is not part of the validated data: a Document never holds it. A project
// whose repository could not be looked up has no entry, and a renderer shows
// that project from its hand written fields alone. A nil Enrichment is valid
// and enriches nothing.
type Enrichment map[string]github.Info

// For returns the enrichment of p, and false when there is none.
func (e Enrichment) For(p ProjectEntry) (github.Info, bool) {
	info, ok := e[p.GithubRepo]
	return info, ok
}

// Enrich looks up every distinct github_repo of the projects in docs, once
// each however many documents name it, and returns what it found. A failed
// lookup, a rate limit and a missing repository each log one warning for that
// repository and leave it out. Enrich never fails: an outage degrades the
// project list and never the render. A cancelled ctx ends the lookups, each
// remaining repository logging its own warning.
func Enrich(ctx context.Context, src RepoSource, log *slog.Logger, docs ...Document) Enrichment {
	out := Enrichment{}
	seen := map[string]bool{}
	for _, doc := range docs {
		for _, p := range doc.Projects {
			if seen[p.GithubRepo] {
				continue
			}
			seen[p.GithubRepo] = true
			info, err := src.Repo(ctx, p.GithubRepo)
			if err != nil {
				log.WarnContext(ctx, "github enrichment skipped, rendering from the hand written fields",
					"repo", p.GithubRepo, "error", err)
				continue
			}
			out[p.GithubRepo] = info
		}
	}
	return out
}
