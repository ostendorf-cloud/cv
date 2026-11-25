package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRenderJSON(t *testing.T) {
	t.Parallel()

	t.Run("a valid pair writes one document per language", func(t *testing.T) {
		t.Parallel()

		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "json", "--no-github", "--dir", "testdata/render-pair", "--out", out}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("code = %d, stderr = %q", code, stderr.String())
		}

		entries, err := os.ReadDir(out)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		if strings.Join(names, ",") != "cv.de.json,cv.en.json" {
			t.Fatalf("files = %v, want exactly cv.de.json and cv.en.json", names)
		}

		de := readFile(t, filepath.Join(out, "cv.de.json"))
		en := readFile(t, filepath.Join(out, "cv.en.json"))
		if !strings.Contains(de, "Kubernetes Deutsch") || strings.Contains(de, "Kubernetes English") {
			t.Errorf("cv.de.json mixes languages:\n%s", de)
		}
		if !strings.Contains(en, "Kubernetes English") || strings.Contains(en, "Kubernetes Deutsch") {
			t.Errorf("cv.en.json mixes languages:\n%s", en)
		}
		if !strings.Contains(de, `"language": "de"`) || !strings.Contains(en, `"language": "en"`) {
			t.Errorf("a document names the wrong language")
		}
	})

	t.Run("two runs write byte identical files", func(t *testing.T) {
		t.Parallel()

		outA := t.TempDir()
		outB := t.TempDir()
		for _, out := range []string{outA, outB} {
			var stdout, stderr bytes.Buffer
			if code := run([]string{"render", "json", "--no-github", "--dir", "testdata/render-pair", "--out", out}, &stdout, &stderr); code != 0 {
				t.Fatalf("code = %d, stderr = %q", code, stderr.String())
			}
		}
		for _, name := range []string{"cv.de.json", "cv.en.json"} {
			if readFile(t, filepath.Join(outA, name)) != readFile(t, filepath.Join(outB, name)) {
				t.Errorf("%s differs between two runs", name)
			}
		}
	})

	t.Run("an invalid pair fails and writes nothing", func(t *testing.T) {
		t.Parallel()

		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "json", "--dir", "testdata/invalid-pair", "--out", out}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "terraform") {
			t.Errorf("stderr = %q, want it to name the drifted id", stderr.String())
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Errorf("the output directory exists after a failed render: %v", err)
		}
	})

	t.Run("a missing artefact name fails with the usage", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		if code := run([]string{"render"}, &stdout, &stderr); code != 2 {
			t.Errorf("code = %d, want 2", code)
		}
		if stderr.String() != renderUsage {
			t.Errorf("stderr = %q, want the render usage", stderr.String())
		}
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
