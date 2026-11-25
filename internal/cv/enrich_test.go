package cv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/cv/internal/github"
)

// fakeSource answers from canned results and records every lookup.
type fakeSource struct {
	info  map[string]github.Info
	err   map[string]error
	asked []string
}

func (f *fakeSource) Repo(_ context.Context, repo string) (github.Info, error) {
	f.asked = append(f.asked, repo)
	if err, ok := f.err[repo]; ok {
		return github.Info{}, err
	}
	return f.info[repo], nil
}

func projectDoc(repos ...string) Document {
	var doc Document
	for i, r := range repos {
		doc.Projects = append(doc.Projects, ProjectEntry{ID: fmt.Sprintf("p%d", i), GithubRepo: r, Description: "Hand written " + r})
	}
	return doc
}

func projectsOf(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var out struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out.Projects
}

func TestEnrich(t *testing.T) {
	t.Parallel()

	good := github.Info{
		Languages:     []github.Language{{Name: "Go", Bytes: 10}},
		LatestRelease: &github.Release{Tag: "v1.0.0", URL: "https://example.com/r"},
	}

	t.Run("a repository named in both documents is looked up once", func(t *testing.T) {
		t.Parallel()
		src := &fakeSource{info: map[string]github.Info{"a/b": good}}
		enr := Enrich(context.Background(), src, slog.New(slog.DiscardHandler), projectDoc("a/b"), projectDoc("a/b"))
		if len(src.asked) != 1 {
			t.Errorf("lookups = %v, want one", src.asked)
		}
		if _, ok := enr.For(ProjectEntry{GithubRepo: "a/b"}); !ok {
			t.Error("no enrichment for a/b")
		}
	})

	t.Run("a failure warns once, leaves the project out and does not stop the others", func(t *testing.T) {
		t.Parallel()
		src := &fakeSource{
			info: map[string]github.Info{"a/ok": good},
			err: map[string]error{
				"a/gone": github.ErrNotFound,
				"a/slow": context.DeadlineExceeded,
				"a/rate": github.ErrRateLimited,
			},
		}
		var logs bytes.Buffer
		log := slog.New(slog.NewTextHandler(&logs, nil))
		doc := projectDoc("a/gone", "a/slow", "a/rate", "a/ok")
		enr := Enrich(context.Background(), src, log, doc, doc)
		if len(enr) != 1 {
			t.Errorf("enrichment = %v, want only a/ok", enr)
		}
		for _, r := range []string{"a/gone", "a/slow", "a/rate"} {
			if n := strings.Count(logs.String(), "repo="+r+" "); n != 1 {
				t.Errorf("%s warned %d times, want 1:\n%s", r, n, logs.String())
			}
		}
		if !strings.Contains(logs.String(), "level=WARN") {
			t.Errorf("not a warning:\n%s", logs.String())
		}
	})

	t.Run("no projects asks nothing", func(t *testing.T) {
		t.Parallel()
		src := &fakeSource{}
		enr := Enrich(context.Background(), src, slog.New(slog.DiscardHandler), Document{})
		if len(enr) != 0 || len(src.asked) != 0 {
			t.Errorf("enr = %v, asked = %v", enr, src.asked)
		}
	})

	t.Run("a nil enrichment enriches nothing", func(t *testing.T) {
		t.Parallel()
		var enr Enrichment
		if _, ok := enr.For(ProjectEntry{GithubRepo: "a/b"}); ok {
			t.Error("a nil enrichment returned data")
		}
	})
}

func TestRenderJSONEnriched(t *testing.T) {
	t.Parallel()

	doc := projectDoc("a/ok", "a/gone", "a/norelease", "a/empty")
	enr := Enrichment{
		"a/ok": {
			Languages:     []github.Language{{Name: "Go", Bytes: 900}, {Name: "Shell", Bytes: 100}},
			LatestRelease: &github.Release{Tag: "v1.2.3", Name: "", URL: "https://example.com/r", PublishedAt: "2026-11-01T10:00:00Z"},
		},
		"a/norelease": {Languages: []github.Language{{Name: "Go", Bytes: 1}}},
		"a/empty":     {},
	}

	data, err := RenderJSONEnriched(doc, "en", enr)
	if err != nil {
		t.Fatal(err)
	}
	projects := projectsOf(t, data)

	t.Run("an enriched project carries languages and the release", func(t *testing.T) {
		t.Parallel()
		p := projects[0]
		langs, _ := p["languages"].([]any)
		if len(langs) != 2 {
			t.Fatalf("languages = %v", p["languages"])
		}
		first, _ := langs[0].(map[string]any)
		if first["name"] != "Go" || first["bytes"] != float64(900) {
			t.Errorf("languages[0] = %v", first)
		}
		rel, _ := p["latest_release"].(map[string]any)
		if rel["tag"] != "v1.2.3" || rel["url"] != "https://example.com/r" || rel["name"] != nil {
			t.Errorf("latest_release = %v (an unset name is null)", p["latest_release"])
		}
		if p["description"] != "Hand written a/ok" {
			t.Errorf("the hand written description changed: %v", p["description"])
		}
	})

	t.Run("a degraded project renders from its hand written fields alone", func(t *testing.T) {
		t.Parallel()
		p := projects[1]
		if _, has := p["languages"]; has {
			t.Errorf("languages present: %v", p)
		}
		if _, has := p["latest_release"]; has {
			t.Errorf("latest_release present: %v", p)
		}
		if p["id"] != "p1" || p["description"] != "Hand written a/gone" {
			t.Errorf("project = %v", p)
		}
	})

	t.Run("a repository without a release has languages only", func(t *testing.T) {
		t.Parallel()
		if _, has := projects[2]["latest_release"]; has {
			t.Errorf("latest_release present: %v", projects[2])
		}
		if _, has := projects[2]["languages"]; !has {
			t.Errorf("languages missing: %v", projects[2])
		}
	})

	t.Run("a repository with no data adds no keys", func(t *testing.T) {
		t.Parallel()
		if len(projects[3]) != 3 {
			t.Errorf("project = %v, want id, github_repo, description", projects[3])
		}
	})

	t.Run("RenderJSON equals an enrichment of nothing", func(t *testing.T) {
		t.Parallel()
		plain, err := RenderJSON(doc, "en")
		if err != nil {
			t.Fatal(err)
		}
		nilEnr, err := RenderJSONEnriched(doc, "en", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(plain, nilEnr) {
			t.Error("RenderJSON and RenderJSONEnriched(nil) differ")
		}
	})
}

func TestEnrich_WarningNamesTheCause(t *testing.T) {
	t.Parallel()

	src := &fakeSource{err: map[string]error{"a/b": fmt.Errorf("wrapped: %w", github.ErrRateLimited)}}
	var logs bytes.Buffer
	Enrich(context.Background(), src, slog.New(slog.NewTextHandler(&logs, nil)), projectDoc("a/b"))
	if !strings.Contains(logs.String(), "rate limited") {
		t.Errorf("the cause is not in the warning:\n%s", logs.String())
	}
}
