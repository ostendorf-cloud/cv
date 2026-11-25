package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// githubFake serves example/alpha and answers 404 for everything else. It
// records the path and Authorization header of every request.
type githubFake struct {
	mu          sync.Mutex
	paths       []string
	auths       []string
	rateLimited bool
}

func (g *githubFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	g.paths = append(g.paths, r.URL.Path)
	g.auths = append(g.auths, r.Header.Get("Authorization"))
	limited := g.rateLimited
	g.mu.Unlock()

	switch {
	case limited:
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	case r.URL.Path == "/repos/example/alpha/languages":
		_, _ = w.Write([]byte(`{"Go": 900, "Shell": 100}`)) // a short write shows up as a failed assertion
	case r.URL.Path == "/repos/example/alpha/releases/latest":
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","html_url":"https://example.com/r"}`)) // as above
	default:
		http.NotFound(w, r)
	}
}

func renderProjects(t *testing.T, g *githubFake, lang string) ([]map[string]any, string, int) {
	t.Helper()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)

	out := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	code := run([]string{"render", "json", "--github-url", srv.URL, "--dir", "testdata/projects-pair", "--out", out}, &stdout, &stderr)
	if code != 0 {
		return nil, stderr.String(), code
	}
	var doc struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(out, "cv."+lang+".json"))), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Projects, stderr.String(), code
}

// These tests set the environment, so none of them runs in parallel.

func TestRenderJSONEnrichesProjects(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN_FILE", "")

	g := &githubFake{}
	for _, lang := range []string{"en", "de"} {
		projects, stderr, code := renderProjects(t, &githubFake{}, lang)
		if code != 0 {
			t.Fatalf("%s: code = %d, stderr = %q", lang, code, stderr)
		}
		if projects[0]["languages"] == nil || projects[0]["latest_release"] == nil {
			t.Errorf("%s: alpha is not enriched: %v", lang, projects[0])
		}
		if _, has := projects[1]["languages"]; has {
			t.Errorf("%s: a missing repository gained languages: %v", lang, projects[1])
		}
		if strings.Count(stderr, "repo=example/gone") != 1 {
			t.Errorf("%s: want one warning for the missing repository, stderr = %q", lang, stderr)
		}
	}

	// One render writes both languages: each repository is fetched once.
	srv := httptest.NewServer(g)
	defer srv.Close()
	var stdout, stderr bytes.Buffer
	args := []string{"render", "json", "--github-url", srv.URL, "--dir", "testdata/projects-pair", "--out", t.TempDir()}
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	// alpha: languages and release. gone: one 404 on languages, no retry.
	if len(g.paths) != 3 {
		t.Errorf("calls = %v, want 3, each repository once for both languages", g.paths)
	}
	for i, a := range g.auths {
		if a != "" {
			t.Errorf("request %d sent Authorization %q with no token configured", i, a)
		}
	}
}

func TestRenderJSONDegradesOnRateLimit(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN_FILE", "")

	g := &githubFake{rateLimited: true}
	projects, stderr, code := renderProjects(t, g, "en")
	if code != 0 {
		t.Fatalf("a rate limit must not fail the render: code = %d, stderr = %q", code, stderr)
	}
	for _, p := range projects {
		if _, has := p["languages"]; has {
			t.Errorf("project %v enriched under a rate limit", p["id"])
		}
		if p["description"] == "" {
			t.Errorf("project %v lost its hand written description", p["id"])
		}
	}
	if !strings.Contains(stderr, "level=WARN") || !strings.Contains(stderr, "rate limited") {
		t.Errorf("stderr = %q, want a rate limit warning", stderr)
	}
	if len(g.paths) != 1 {
		t.Errorf("calls = %v, want 1: nothing after the rate limit", g.paths)
	}
}

func TestRenderJSONNoGithubMakesNoCall(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN_FILE", "")

	g := &githubFake{}
	srv := httptest.NewServer(g)
	defer srv.Close()
	var stdout, stderr bytes.Buffer
	args := []string{"render", "json", "--no-github", "--github-url", srv.URL, "--dir", "testdata/projects-pair", "--out", t.TempDir()}
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if len(g.paths) != 0 {
		t.Errorf("calls = %v, want none", g.paths)
	}
}

func TestRenderJSONToken(t *testing.T) {
	const token = "ghp_notarealtoken0123456789"

	t.Run("the token from the environment is sent and never logged", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		t.Setenv("GITHUB_TOKEN_FILE", "")
		g := &githubFake{}
		_, stderr, code := renderProjects(t, g, "en")
		if code != 0 {
			t.Fatalf("code = %d, stderr = %q", code, stderr)
		}
		if len(g.auths) == 0 || g.auths[0] != "Bearer "+token {
			t.Errorf("auths = %v", g.auths)
		}
		if strings.Contains(stderr, token) {
			t.Errorf("stderr carries the token: %q", stderr)
		}
	})

	t.Run("the token file wins and a rate limited render never logs it", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GITHUB_TOKEN", "ghp_other")
		t.Setenv("GITHUB_TOKEN_FILE", path)
		g := &githubFake{rateLimited: true}
		_, stderr, code := renderProjects(t, g, "en")
		if code != 0 {
			t.Fatalf("code = %d, stderr = %q", code, stderr)
		}
		if g.auths[0] != "Bearer "+token {
			t.Errorf("the file did not win: %v", g.auths)
		}
		if strings.Contains(stderr, token) || strings.Contains(stderr, "ghp_other") {
			t.Errorf("stderr carries a token: %q", stderr)
		}
	})

	t.Run("an unreadable token file fails the render", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GITHUB_TOKEN_FILE", filepath.Join(t.TempDir(), "missing"))
		_, stderr, code := renderProjects(t, &githubFake{}, "en")
		if code != 1 || !strings.Contains(stderr, "GITHUB_TOKEN_FILE") {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
}
