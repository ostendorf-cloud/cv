package cvd

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ostendorf-cloud/cv/internal/cv"
	"github.com/ostendorf-cloud/cv/internal/github"
)

// projectsDir holds a pair with two projects, example/alpha and example/beta.
const projectsDir = "../../cmd/cv/testdata/suggest-pair"

// fakeRepo is what the fake GitHub knows about one repository. An empty
// release means the repository has none.
type fakeRepo struct {
	languages map[string]int64
	release   string
}

// fakeGitHub is a GitHub REST API stand in. It records the Authorization
// header of every call and can be switched to answer 500 to everything.
type fakeGitHub struct {
	mu    sync.Mutex
	down  bool
	repos map[string]fakeRepo
	auth  []string
}

func (f *fakeGitHub) set(repo string, r fakeRepo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos[repo] = r
}

func (f *fakeGitHub) setDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

func (f *fakeGitHub) authHeaders() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.auth...)
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	if f.down {
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	}
	// /repos/<owner>/<name>/languages or /repos/<owner>/<name>/releases/latest
	rest := strings.TrimPrefix(r.URL.Path, "/repos/")
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	repo, ok := f.repos[parts[0]+"/"+parts[1]]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch parts[2] {
	case "languages":
		_ = json.NewEncoder(w).Encode(repo.languages)
	case "releases/latest":
		if repo.release == "" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"tag_name":     repo.release,
			"name":         "Release " + repo.release,
			"html_url":     "https://github.com/" + parts[0] + "/" + parts[1] + "/releases/tag/" + repo.release,
			"published_at": "2026-11-01T10:00:00Z",
		})
	default:
		http.NotFound(w, r)
	}
}

// newFakeGitHub starts the fake with a full alpha and a missing beta.
func newFakeGitHub(t *testing.T) (*fakeGitHub, *httptest.Server) {
	t.Helper()
	f := &fakeGitHub{repos: map[string]fakeRepo{
		"example/alpha": {languages: map[string]int64{"Go": 900, "Shell": 100}, release: "v1.0.0"},
	}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

// githubApp is an App with the projects pair loaded and GitHub wired to srv.
func githubApp(t *testing.T, srv *httptest.Server, token string, logger *slog.Logger) *App {
	t.Helper()
	source := func() cv.RepoSource {
		return github.New(github.Config{BaseURL: srv.URL, Token: token, HTTPClient: srv.Client()})
	}
	app := New(logger, WithGitHub(source, time.Hour))
	if err := app.Load(projectsDir); err != nil {
		t.Fatalf("Load(%s) error = %v", projectsDir, err)
	}
	return app
}

func projectsIn(t *testing.T, body []byte) map[string]map[string]any {
	t.Helper()
	var doc struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]any{}
	for _, p := range doc.Projects {
		id, ok := p["id"].(string)
		if !ok {
			t.Fatalf("project id = %v, want a string", p["id"])
		}
		out[id] = p
	}
	return out
}

func TestRefreshServesLanguagesAndLatestRelease(t *testing.T) {
	t.Parallel()

	_, srv := newFakeGitHub(t)
	app := githubApp(t, srv, "", discardLogger())
	app.Refresh(t.Context())

	for _, lang := range cv.Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			rec := get(t, app, http.MethodGet, "/api/v1/cv?lang="+lang, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			alpha := projectsIn(t, rec.Body.Bytes())["alpha"]

			langs, ok := alpha["languages"].([]any)
			if !ok || len(langs) != 2 {
				t.Fatalf("alpha languages = %v, want two entries", alpha["languages"])
			}
			first, _ := langs[0].(map[string]any)
			if first["name"] != "Go" || first["bytes"] != float64(900) {
				t.Errorf("first language = %v, want Go with 900 bytes", langs[0])
			}
			release, ok := alpha["latest_release"].(map[string]any)
			if !ok || release["tag"] != "v1.0.0" {
				t.Errorf("alpha latest_release = %v, want tag v1.0.0", alpha["latest_release"])
			}
		})
	}
}

func TestRefreshOmitsTheKeysOfAProjectWithoutData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// beta is what the fake GitHub knows about example/beta, nil for nothing.
		beta        *fakeRepo
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name:       "a repository GitHub does not know",
			beta:       nil,
			wantAbsent: []string{"languages", "latest_release"},
		},
		{
			name:        "a repository without a release",
			beta:        &fakeRepo{languages: map[string]int64{"Python": 5}},
			wantPresent: []string{"languages"},
			wantAbsent:  []string{"latest_release"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, srv := newFakeGitHub(t)
			if tt.beta != nil {
				f.set("example/beta", *tt.beta)
			}
			app := githubApp(t, srv, "", discardLogger())
			app.Refresh(t.Context())

			for _, lang := range cv.Languages {
				rec := get(t, app, http.MethodGet, "/api/v1/cv?lang="+lang, nil)
				beta := projectsIn(t, rec.Body.Bytes())["beta"]
				for _, key := range tt.wantPresent {
					if _, ok := beta[key]; !ok {
						t.Errorf("%s: beta has no %s key, want it", lang, key)
					}
				}
				for _, key := range tt.wantAbsent {
					if _, ok := beta[key]; ok {
						t.Errorf("%s: beta has a %s key, want it omitted", lang, key)
					}
				}
			}
		})
	}
}

func TestRefreshWithoutAnyDataKeepsTheDocumentWhenGitHubFails(t *testing.T) {
	t.Parallel()

	f, srv := newFakeGitHub(t)
	f.setDown(true)
	app := githubApp(t, srv, "", discardLogger())
	before := get(t, app, http.MethodGet, "/api/v1/cv", nil)

	app.Refresh(t.Context())

	after := get(t, app, http.MethodGet, "/api/v1/cv", nil)
	if after.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 although GitHub is down", after.Code)
	}
	if after.Body.String() != before.Body.String() {
		t.Error("the document changed although no GitHub call succeeded")
	}
	for id, p := range projectsIn(t, after.Body.Bytes()) {
		for _, key := range []string{"languages", "latest_release"} {
			if _, ok := p[key]; ok {
				t.Errorf("project %s has a %s key, want it omitted without data", id, key)
			}
		}
	}
}

func TestRefreshKeepsTheLastGoodDataWhenGitHubFails(t *testing.T) {
	t.Parallel()

	f, srv := newFakeGitHub(t)
	app := githubApp(t, srv, "", discardLogger())
	app.Refresh(t.Context())
	good := get(t, app, http.MethodGet, "/api/v1/cv", nil)

	f.setDown(true)
	app.Refresh(t.Context())

	after := get(t, app, http.MethodGet, "/api/v1/cv", nil)
	if after.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 although GitHub is down", after.Code)
	}
	if after.Body.String() != good.Body.String() {
		t.Error("the document changed after a failed refresh, want the last good data")
	}
	if after.Header().Get("ETag") != good.Header().Get("ETag") {
		t.Error("ETag changed after a failed refresh")
	}
	if _, ok := projectsIn(t, after.Body.Bytes())["alpha"]["languages"]; !ok {
		t.Error("alpha lost its languages after a failed refresh")
	}
}

func TestRefreshKeepsTheDataOfARepositoryThatFailsAlone(t *testing.T) {
	t.Parallel()

	f, srv := newFakeGitHub(t)
	f.set("example/beta", fakeRepo{languages: map[string]int64{"Python": 5}})
	app := githubApp(t, srv, "", discardLogger())
	app.Refresh(t.Context())

	// alpha disappears (404), beta moves on. alpha keeps its old data.
	f.mu.Lock()
	delete(f.repos, "example/alpha")
	f.repos["example/beta"] = fakeRepo{languages: map[string]int64{"Python": 5, "Go": 1}, release: "v2.0.0"}
	f.mu.Unlock()
	app.Refresh(t.Context())

	projects := projectsIn(t, get(t, app, http.MethodGet, "/api/v1/cv", nil).Body.Bytes())
	alphaRelease, _ := projects["alpha"]["latest_release"].(map[string]any)
	if alphaRelease["tag"] != "v1.0.0" {
		t.Errorf("alpha latest_release = %v, want the kept v1.0.0", projects["alpha"]["latest_release"])
	}
	betaRelease, _ := projects["beta"]["latest_release"].(map[string]any)
	if betaRelease["tag"] != "v2.0.0" {
		t.Errorf("beta latest_release = %v, want the fresh v2.0.0", projects["beta"]["latest_release"])
	}
}

func TestETagChangesWithTheGitHubData(t *testing.T) {
	t.Parallel()

	f, srv := newFakeGitHub(t)
	app := githubApp(t, srv, "", discardLogger())
	bare := get(t, app, http.MethodGet, "/api/v1/cv", nil).Header().Get("ETag")

	app.Refresh(t.Context())
	first := get(t, app, http.MethodGet, "/api/v1/cv", nil).Header().Get("ETag")
	if first == bare {
		t.Fatal("ETag did not change when the GitHub data arrived")
	}

	// The same data again keeps the tag, and a 304 still works.
	app.Refresh(t.Context())
	again := get(t, app, http.MethodGet, "/api/v1/cv", nil)
	if again.Header().Get("ETag") != first {
		t.Error("ETag changed although the GitHub data did not")
	}
	rec := get(t, app, http.MethodGet, "/api/v1/cv", http.Header{"If-None-Match": {first}})
	if rec.Code != http.StatusNotModified {
		t.Errorf("If-None-Match with the current tag: status = %d, want 304", rec.Code)
	}

	// New data gives a new tag. The old tag no longer matches and the new one does.
	f.set("example/alpha", fakeRepo{languages: map[string]int64{"Go": 900, "Shell": 100}, release: "v1.1.0"})
	app.Refresh(t.Context())
	changed := get(t, app, http.MethodGet, "/api/v1/cv", nil).Header().Get("ETag")
	if changed == first {
		t.Error("ETag did not change when the latest release changed")
	}
	if rec := get(t, app, http.MethodGet, "/api/v1/cv", http.Header{"If-None-Match": {first}}); rec.Code != http.StatusOK {
		t.Errorf("If-None-Match with the old tag: status = %d, want 200", rec.Code)
	}
	if rec := get(t, app, http.MethodGet, "/api/v1/cv", http.Header{"If-None-Match": {changed}}); rec.Code != http.StatusNotModified {
		t.Errorf("If-None-Match with the new tag: status = %d, want 304", rec.Code)
	}
}

func TestTokenIsSentWhenSetAndNeverLogged(t *testing.T) {
	t.Parallel()

	const token = "ghs_fixture_token_value"
	tests := []struct {
		name     string
		token    string
		wantAuth string
	}{
		{"a token is sent as a bearer token", token, "Bearer " + token},
		{"no token sends no credential", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, srv := newFakeGitHub(t)
			var logs bytes.Buffer
			app := githubApp(t, srv, tt.token, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))

			// One good refresh and one failing refresh, so both log paths run.
			app.Refresh(t.Context())
			f.setDown(true)
			app.Refresh(t.Context())

			auth := f.authHeaders()
			if len(auth) == 0 {
				t.Fatal("the fake GitHub saw no call")
			}
			for _, got := range auth {
				if got != tt.wantAuth {
					t.Errorf("Authorization = %q, want %q", got, tt.wantAuth)
				}
			}
			// Without a token there is nothing that could leak.
			if tt.token == "" {
				return
			}
			for _, secret := range []string{tt.token, "Bearer"} {
				if strings.Contains(logs.String(), secret) {
					t.Errorf("the log carries %q:\n%s", secret, logs.String())
				}
			}
		})
	}
}

func TestRefreshWithoutAGitHubSourceChangesNothing(t *testing.T) {
	t.Parallel()

	app := New(discardLogger())
	if err := app.Load(projectsDir); err != nil {
		t.Fatal(err)
	}
	before := get(t, app, http.MethodGet, "/api/v1/cv", nil).Body.String()

	app.Refresh(t.Context())

	if got := get(t, app, http.MethodGet, "/api/v1/cv", nil).Body.String(); got != before {
		t.Error("the document changed without a GitHub source")
	}
}

func TestRefreshBeforeLoadDoesNothing(t *testing.T) {
	t.Parallel()

	calls := 0
	app := New(discardLogger(), WithGitHub(func() cv.RepoSource {
		calls++
		return nil
	}, time.Hour))

	app.Refresh(t.Context())

	if calls != 0 {
		t.Errorf("the source was built %d times before Load, want 0", calls)
	}
	if rec := get(t, app, http.MethodGet, "/api/v1/cv", nil); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 before Load", rec.Code)
	}
}

func TestRefreshEveryRefreshesUntilTheContextEnds(t *testing.T) {
	t.Parallel()

	_, srv := newFakeGitHub(t)
	built := make(chan struct{}, 16)
	source := func() cv.RepoSource {
		select {
		case built <- struct{}{}:
		default:
		}
		return github.New(github.Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	}
	app := New(discardLogger(), WithGitHub(source, time.Millisecond))
	if err := app.Load(projectsDir); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.RefreshEvery(ctx)
	}()

	for range 2 {
		select {
		case <-built:
		case <-time.After(10 * time.Second):
			t.Fatal("no refresh within 10 seconds")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("RefreshEvery did not return after its context ended")
	}

	if _, ok := projectsIn(t, get(t, app, http.MethodGet, "/api/v1/cv", nil).Body.Bytes())["alpha"]["languages"]; !ok {
		t.Error("alpha has no languages after the periodic refresh")
	}
}

// TestRefreshEveryRefreshesAtOnce pins that the first lookup does not wait a
// whole ttl: cmd/cvd starts RefreshEvery after the server listens and expects
// the first data right away.
func TestRefreshEveryRefreshesAtOnce(t *testing.T) {
	t.Parallel()

	_, srv := newFakeGitHub(t)
	app := githubApp(t, srv, "", discardLogger()) // ttl is one hour
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.RefreshEvery(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	deadline := time.Now().Add(10 * time.Second)
	for {
		body := get(t, app, http.MethodGet, "/api/v1/cv", nil).Body.Bytes()
		if _, ok := projectsIn(t, body)["alpha"]["languages"]; ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no GitHub data within 10 seconds, want the first refresh at once")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestRouteAnswersWhileASlowRefreshRuns pins the startup contract: a GitHub
// that holds every call and then answers 503 never keeps the route from
// answering, so the liveness probe of the chart never waits on it.
func TestRouteAnswersWhileASlowRefreshRuns(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{}, 1)
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-gate:
		case <-r.Context().Done():
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(release) // runs before srv.Close, so no handler outlives the test
	app := githubApp(t, srv, "", discardLogger())
	bare := get(t, app, http.MethodGet, "/api/v1/cv", nil).Body.String()

	ctx, cancel := context.WithCancel(t.Context())
	refreshed := make(chan struct{})
	go func() {
		defer close(refreshed)
		app.RefreshEvery(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		release()
		<-refreshed
	})

	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the refresh never called GitHub")
	}

	// The refresh now sits inside a GitHub call. The route must still answer.
	answered := make(chan *httptest.ResponseRecorder, 1)
	go func() { answered <- get(t, app, http.MethodGet, "/api/v1/cv", nil) }()
	select {
	case rec := <-answered:
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 while the refresh runs", rec.Code)
		}
		if rec.Body.String() != bare {
			t.Error("the document changed before any GitHub data arrived")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the route did not answer while the refresh was running")
	}

	// The slow call is still held, so the answer came before the refresh ended.
	select {
	case <-refreshed:
		t.Fatal("the refresh ended before the held GitHub call was released")
	default:
	}
	release()
	cancel()
	select {
	case <-refreshed:
	case <-time.After(10 * time.Second):
		t.Fatal("RefreshEvery did not return after the 503 answers")
	}
	rec := get(t, app, http.MethodGet, "/api/v1/cv", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != bare {
		t.Errorf("after the failed refresh: status = %d, same document = %t, want 200 and true",
			rec.Code, rec.Body.String() == bare)
	}
}

// TestRouteIsRaceFreeDuringRefresh reads the route from several goroutines
// while refreshes swap the documents. It is meant for -race: the handlers take
// no lock and Refresh holds one.
func TestRouteIsRaceFreeDuringRefresh(t *testing.T) {
	t.Parallel()

	f, srv := newFakeGitHub(t)
	app := githubApp(t, srv, "", discardLogger())

	var readers sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				rec := get(t, app, http.MethodGet, "/api/v1/cv", nil)
				if rec.Code != http.StatusOK {
					t.Errorf("status = %d, want 200 during a refresh", rec.Code)
					return
				}
				if !json.Valid(rec.Body.Bytes()) {
					t.Error("the body is not valid JSON during a refresh")
					return
				}
			}
		}()
	}

	// Each round changes the release, so every refresh really swaps the documents.
	for i := range 20 {
		f.set("example/alpha", fakeRepo{
			languages: map[string]int64{"Go": 900},
			release:   "v1." + string(rune('0'+i%10)) + ".0",
		})
		app.Refresh(t.Context())
	}
	close(stop)
	readers.Wait()
}

func TestRefreshEveryReturnsAtOnceWithoutASourceOrATTL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []Option
	}{
		{"no source", nil},
		{"a ttl that is not positive", []Option{WithGitHub(func() cv.RepoSource { return nil }, 0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := New(discardLogger(), tt.opts...)
			done := make(chan struct{})
			go func() {
				defer close(done)
				app.RefreshEvery(t.Context())
			}()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("RefreshEvery did not return")
			}
		})
	}
}
