package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fake is a GitHub API double. It records every request and answers from
// route, keyed by request path.
type fake struct {
	mu    sync.Mutex
	paths []string
	auth  []string
	route map[string]http.HandlerFunc
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path)
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	h, ok := f.route[r.URL.Path]
	f.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	h(w, r)
}

func (f *fake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.paths)
}

func body(status int, s string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(s)) // a failed write surfaces as a failed test read
	}
}

const (
	languagesPath = "/repos/example/alpha/languages"
	releasePath   = "/repos/example/alpha/releases/latest"
)

func okRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		languagesPath: body(200, `{"Go": 900, "Shell": 100, "Makefile": 100}`),
		releasePath: body(200, `{"tag_name":"v1.2.3","name":"Release 1.2.3",`+
			`"html_url":"https://github.com/example/alpha/releases/tag/v1.2.3","published_at":"2026-11-01T10:00:00Z"}`),
	}
}

func newClient(t *testing.T, routes map[string]http.HandlerFunc, cfg Config) (*Client, *fake) {
	t.Helper()
	f := &fake{route: routes}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL
	return New(cfg), f
}

func TestClient_Repo(t *testing.T) {
	t.Parallel()

	t.Run("success returns languages and the latest release", func(t *testing.T) {
		t.Parallel()
		c, _ := newClient(t, okRoutes(), Config{})
		info, err := c.Repo(context.Background(), "example/alpha")
		if err != nil {
			t.Fatal(err)
		}
		want := []Language{{"Go", 900}, {"Makefile", 100}, {"Shell", 100}}
		if len(info.Languages) != len(want) {
			t.Fatalf("languages = %v, want %v", info.Languages, want)
		}
		for i := range want {
			if info.Languages[i] != want[i] {
				t.Errorf("languages[%d] = %v, want %v (bytes descending, then name)", i, info.Languages[i], want[i])
			}
		}
		if info.LatestRelease == nil || info.LatestRelease.Tag != "v1.2.3" || info.LatestRelease.PublishedAt != "2026-11-01T10:00:00Z" {
			t.Errorf("release = %+v", info.LatestRelease)
		}
	})

	t.Run("a repository without a release has languages and no release", func(t *testing.T) {
		t.Parallel()
		routes := okRoutes()
		delete(routes, releasePath)
		c, _ := newClient(t, routes, Config{})
		info, err := c.Repo(context.Background(), "example/alpha")
		if err != nil {
			t.Fatal(err)
		}
		if info.LatestRelease != nil || len(info.Languages) == 0 {
			t.Errorf("info = %+v", info)
		}
	})

	t.Run("a missing repository is reported once and never retried", func(t *testing.T) {
		t.Parallel()
		c, f := newClient(t, map[string]http.HandlerFunc{}, Config{})
		for range 3 {
			_, err := c.Repo(context.Background(), "example/alpha")
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		}
		if n := f.count(); n != 1 {
			t.Errorf("calls = %d, want 1: the 404 must neither be retried nor re-asked", n)
		}
	})

	t.Run("a primary rate limit stops every further call", func(t *testing.T) {
		t.Parallel()
		routes := map[string]http.HandlerFunc{
			languagesPath: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(http.StatusForbidden)
			},
		}
		c, f := newClient(t, routes, Config{})
		for _, repo := range []string{"example/alpha", "example/beta", "example/gamma"} {
			if _, err := c.Repo(context.Background(), repo); !errors.Is(err, ErrRateLimited) {
				t.Fatalf("%s: err = %v, want ErrRateLimited", repo, err)
			}
		}
		if n := f.count(); n != 1 {
			t.Errorf("calls = %d, want 1: after a rate limit no call is made", n)
		}
	})

	t.Run("429 with a retry header is a rate limit", func(t *testing.T) {
		t.Parallel()
		routes := map[string]http.HandlerFunc{
			languagesPath: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
			},
		}
		c, _ := newClient(t, routes, Config{})
		if _, err := c.Repo(context.Background(), "example/alpha"); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
	})

	t.Run("403 with only a retry header is a rate limit", func(t *testing.T) {
		t.Parallel()
		routes := map[string]http.HandlerFunc{
			languagesPath: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(http.StatusForbidden)
			},
		}
		c, _ := newClient(t, routes, Config{})
		if _, err := c.Repo(context.Background(), "example/alpha"); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
	})

	t.Run("403 whose body names a rate limit is a rate limit", func(t *testing.T) {
		t.Parallel()
		routes := map[string]http.HandlerFunc{
			languagesPath: body(403, `{"message":"You have exceeded a secondary Rate Limit. Please wait."}`),
		}
		c, f := newClient(t, routes, Config{})
		for _, repo := range []string{"example/alpha", "example/beta"} {
			if _, err := c.Repo(context.Background(), repo); !errors.Is(err, ErrRateLimited) {
				t.Fatalf("%s: err = %v, want ErrRateLimited", repo, err)
			}
		}
		if n := f.count(); n != 1 {
			t.Errorf("calls = %d, want 1", n)
		}
	})

	t.Run("equal bytes sort by name, whatever the map order", func(t *testing.T) {
		t.Parallel()
		routes := okRoutes()
		routes[languagesPath] = body(200, `{"H":5,"G":5,"F":5,"E":5,"D":5,"C":5,"B":5,"A":5}`)
		c, _ := newClient(t, routes, Config{})
		info, err := c.Repo(context.Background(), "example/alpha")
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, l := range info.Languages {
			names = append(names, l.Name)
		}
		if got := strings.Join(names, ""); got != "ABCDEFGH" {
			t.Errorf("order = %s, want ABCDEFGH", got)
		}
	})

	t.Run("a release without a tag keeps the languages", func(t *testing.T) {
		t.Parallel()
		routes := okRoutes()
		routes[releasePath] = body(200, `{"name":"x"}`)
		c, _ := newClient(t, routes, Config{})
		info, err := c.Repo(context.Background(), "example/alpha")
		if err != nil {
			t.Fatal(err)
		}
		if info.LatestRelease != nil || len(info.Languages) == 0 {
			t.Errorf("info = %+v, want languages and no release", info)
		}
	})

	t.Run("the first timeout stops every further call", func(t *testing.T) {
		t.Parallel()
		release := make(chan struct{})
		routes := map[string]http.HandlerFunc{
			languagesPath: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-release:
				case <-r.Context().Done():
				}
			},
		}
		c, f := newClient(t, routes, Config{CallTimeout: 50 * time.Millisecond})
		t.Cleanup(func() { close(release) })
		for _, repo := range []string{"example/alpha", "example/beta", "example/gamma"} {
			if _, err := c.Repo(context.Background(), repo); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("%s: err = %v, want context.DeadlineExceeded", repo, err)
			}
		}
		if n := f.count(); n != 1 {
			t.Errorf("calls = %d, want 1: a dead upstream costs one timeout", n)
		}
	})

	t.Run("a refused connection stops every further call", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		c := New(Config{BaseURL: url})
		for _, repo := range []string{"example/alpha", "example/beta"} {
			if _, err := c.Repo(context.Background(), repo); err == nil {
				t.Fatalf("%s: want an error", repo)
			}
		}
		if c.calls != 1 {
			t.Errorf("calls = %d, want 1", c.calls)
		}
	})

	t.Run("403 without rate limit headers is not a rate limit", func(t *testing.T) {
		t.Parallel()
		c, _ := newClient(t, map[string]http.HandlerFunc{languagesPath: body(403, `{}`)}, Config{})
		_, err := c.Repo(context.Background(), "example/alpha")
		if !errors.Is(err, ErrUnexpectedStatus) || errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v, want ErrUnexpectedStatus only", err)
		}
	})

	t.Run("a server error is an unexpected status", func(t *testing.T) {
		t.Parallel()
		c, _ := newClient(t, map[string]http.HandlerFunc{languagesPath: body(502, ``)}, Config{})
		if _, err := c.Repo(context.Background(), "example/alpha"); !errors.Is(err, ErrUnexpectedStatus) {
			t.Fatalf("err = %v, want ErrUnexpectedStatus", err)
		}
	})

	t.Run("a slow call times out", func(t *testing.T) {
		t.Parallel()
		release := make(chan struct{})
		routes := map[string]http.HandlerFunc{
			languagesPath: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-release:
				case <-r.Context().Done():
				}
			},
		}
		c, _ := newClient(t, routes, Config{CallTimeout: 50 * time.Millisecond})
		t.Cleanup(func() { close(release) })
		_, err := c.Repo(context.Background(), "example/alpha")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("malformed JSON is reported", func(t *testing.T) {
		t.Parallel()
		for name, routes := range map[string]map[string]http.HandlerFunc{
			"languages": {languagesPath: body(200, `{"Go": `)},
			"release": {
				languagesPath: body(200, `{"Go": 1}`),
				releasePath:   body(200, `<html>`),
			},
			"oversized body": {
				// More than 1 MiB of leading whitespace before an otherwise valid value.
				languagesPath: body(200, strings.Repeat(" ", maxBodyBytes+1)+`{"Go": 1}`),
			},
			"languages of the wrong type": {languagesPath: body(200, `{"Go": "many"}`)},
		} {
			c, _ := newClient(t, routes, Config{})
			if _, err := c.Repo(context.Background(), "example/alpha"); !errors.Is(err, ErrMalformed) {
				t.Errorf("%s: err = %v, want ErrMalformed", name, err)
			}
		}
	})

	t.Run("the call budget bounds the number of calls", func(t *testing.T) {
		t.Parallel()
		c, f := newClient(t, okRoutes(), Config{MaxCalls: 2})
		if _, err := c.Repo(context.Background(), "example/alpha"); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Repo(context.Background(), "example/beta"); !errors.Is(err, ErrCallBudget) {
			t.Fatalf("err = %v, want ErrCallBudget", err)
		}
		if n := f.count(); n != 2 {
			t.Errorf("calls = %d, want 2", n)
		}
	})

	t.Run("a second lookup is served from the cache", func(t *testing.T) {
		t.Parallel()
		c, f := newClient(t, okRoutes(), Config{})
		first, err := c.Repo(context.Background(), "example/alpha")
		if err != nil {
			t.Fatal(err)
		}
		after := f.count()
		second, err := c.Repo(context.Background(), "example/alpha")
		if err != nil {
			t.Fatal(err)
		}
		if f.count() != after {
			t.Errorf("the second lookup made %d more calls, want 0", f.count()-after)
		}
		if len(second.Languages) != len(first.Languages) {
			t.Errorf("cached info differs: %+v vs %+v", second, first)
		}
	})

	t.Run("an invalid name makes no call", func(t *testing.T) {
		t.Parallel()
		c, f := newClient(t, okRoutes(), Config{})
		for _, repo := range []string{"", "alpha", "a/b/c", "../x/y", "example/alpha?x=1", "example/al pha", "example/.", "example/..", "example/..."} {
			if _, err := c.Repo(context.Background(), repo); !errors.Is(err, ErrInvalidRepo) {
				t.Errorf("%q: err = %v, want ErrInvalidRepo", repo, err)
			}
		}
		if f.count() != 0 {
			t.Errorf("calls = %d, want 0", f.count())
		}
	})

	t.Run("a cancelled context is not cached", func(t *testing.T) {
		t.Parallel()
		c, f := newClient(t, okRoutes(), Config{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := c.Repo(ctx, "example/alpha"); err == nil {
			t.Fatal("want an error from a cancelled context")
		}
		if _, err := c.Repo(context.Background(), "example/alpha"); err != nil {
			t.Fatalf("a later lookup must work: %v", err)
		}
		if f.count() == 0 {
			t.Error("the later lookup made no call")
		}
	})
}

func TestClient_Token(t *testing.T) {
	t.Parallel()

	t.Run("no token sends no Authorization header", func(t *testing.T) {
		t.Parallel()
		c, f := newClient(t, okRoutes(), Config{})
		if _, err := c.Repo(context.Background(), "example/alpha"); err != nil {
			t.Fatal(err)
		}
		for i, a := range f.auth {
			if a != "" {
				t.Errorf("request %d carried Authorization %q", i, a)
			}
		}
		if f.count() == 0 {
			t.Fatal("no request seen")
		}
	})

	t.Run("a token is sent as a bearer token", func(t *testing.T) {
		t.Parallel()
		c, f := newClient(t, okRoutes(), Config{Token: "ghp_faketoken"})
		if _, err := c.Repo(context.Background(), "example/alpha"); err != nil {
			t.Fatal(err)
		}
		for i, a := range f.auth {
			if a != "Bearer ghp_faketoken" {
				t.Errorf("request %d Authorization = %q", i, a)
			}
		}
	})

	t.Run("no error carries the token", func(t *testing.T) {
		t.Parallel()
		const token = "ghp_secretvalue123"
		cases := map[string]map[string]http.HandlerFunc{
			"not found":    {},
			"rate limited": {languagesPath: body(429, ``)},
			"server error": {languagesPath: body(500, token)},
			"malformed":    {languagesPath: body(200, token)},
		}
		for name, routes := range cases {
			c, _ := newClient(t, routes, Config{Token: token})
			_, err := c.Repo(context.Background(), "example/alpha")
			if err == nil {
				t.Fatalf("%s: want an error", name)
			}
			if strings.Contains(err.Error(), token) {
				t.Errorf("%s: the error carries the token: %v", name, err)
			}
		}
	})
}

func TestTokenFromEnv(t *testing.T) {
	t.Parallel()

	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	files := func(m map[string]string) func(string) ([]byte, error) {
		return func(p string) ([]byte, error) {
			s, ok := m[p]
			if !ok {
				return nil, errors.New("no such file")
			}
			return []byte(s), nil
		}
	}

	cases := []struct {
		name    string
		env     map[string]string
		files   map[string]string
		want    string
		wantErr bool
	}{
		{name: "nothing set", want: ""},
		{name: "plain variable", env: map[string]string{"GITHUB_TOKEN": " tok1 \n"}, want: "tok1"},
		{name: "file variable", env: map[string]string{"GITHUB_TOKEN_FILE": "/run/t"}, files: map[string]string{"/run/t": "tok2\n"}, want: "tok2"},
		{name: "file wins over plain", env: map[string]string{"GITHUB_TOKEN": "plain", "GITHUB_TOKEN_FILE": "/run/t"}, files: map[string]string{"/run/t": "fromfile"}, want: "fromfile"},
		{name: "unreadable file", env: map[string]string{"GITHUB_TOKEN_FILE": "/missing"}, wantErr: true},
		{name: "empty file", env: map[string]string{"GITHUB_TOKEN": "plain", "GITHUB_TOKEN_FILE": "/run/t"}, files: map[string]string{"/run/t": " \n"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := TokenFromEnv(env(tc.env), files(tc.files))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("token = %q, want %q", got, tc.want)
			}
		})
	}
}
