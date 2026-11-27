package main

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// suggestFake reports Go and Rust for alpha and Python for beta.
func suggestFake(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/repos/example/alpha/languages":
		_, _ = w.Write([]byte(`{"Go": 900, "Rust": 100}`)) // a short write shows up as a failed assertion
	case "/repos/example/beta/languages":
		_, _ = w.Write([]byte(`{"Python": 500}`)) // as above
	default:
		http.NotFound(w, r)
	}
}

// snapshot returns the content of every file under dir, keyed by path.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		out[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// copyDir copies the files of src, one level deep, into a new temp directory.
func copyDir(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func TestRunSuggestSkills(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN_FILE", "")

	srv := httptest.NewServer(http.HandlerFunc(suggestFake))
	t.Cleanup(srv.Close)

	// The copy is the data directory: its files and the directory itself are
	// snapshotted, so any write, create or removal under it fails the test.
	dir := copyDir(t, "testdata/suggest-pair")
	before := snapshot(t, dir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) // restore so TempDir can remove it

	var stdout, stderr bytes.Buffer
	code := run([]string{"suggest-skills", "--github-url", srv.URL, "--dir", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{
		"+ Rust    used by alpha\n",
		"+ Python    used by beta\n",
		"- go: evidence project beta\n",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}

	after := snapshot(t, dir)
	if len(after) != len(before) {
		t.Errorf("files under data dir: %d before, %d after", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s changed", path)
		}
	}
}

func TestRunSuggestSkillsNothingToSuggest(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN_FILE", "")

	// Every repository is missing: nothing is looked up, nothing is suggested.
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	var stdout, stderr bytes.Buffer
	code := run([]string{"suggest-skills", "--github-url", srv.URL, "--dir", "testdata/suggest-pair"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "nothing was compared") ||
		strings.Contains(stdout.String(), "nothing to suggest") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunSuggestSkillsAllMatch(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN_FILE", "")

	// Both repositories are read and report only Go, which a skill claims.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/example/alpha/languages", "/repos/example/beta/languages":
			_, _ = w.Write([]byte(`{"Go": 900}`)) // a short write shows up as a failed assertion
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	var stdout, stderr bytes.Buffer
	code := run([]string{"suggest-skills", "--github-url", srv.URL, "--dir", "testdata/suggest-pair"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "nothing to suggest") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunSuggestSkillsRefusals(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN_FILE", "")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"invalid data", []string{"suggest-skills", "--dir", "testdata/invalid-pair"}, 1},
		{"no github", []string{"suggest-skills", "--no-github", "--dir", "testdata/suggest-pair"}, 2},
		{"unknown flag", []string{"suggest-skills", "--bogus"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != tt.want {
				t.Errorf("code = %d, want %d, stderr = %q", code, tt.want, stderr.String())
			}
		})
	}
}
