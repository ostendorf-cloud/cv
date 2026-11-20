package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTypst writes an executable stand-in for typst into a temporary
// directory and returns its path. "typst compile in out" copies in to out, so
// the test can read back the source the CLI handed over.
func fakeTypst(t *testing.T, script string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunRenderPDF(t *testing.T) {
	t.Parallel()

	t.Run("a valid pair writes one pdf per language", func(t *testing.T) {
		t.Parallel()

		bin := fakeTypst(t, `[ "$1" = compile ] && cp "$2" "$3"`)
		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "pdf", "--dir", "testdata/render-pair", "--out", out, "--typst", bin}, &stdout, &stderr)
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
		if strings.Join(names, ",") != "cv.de.pdf,cv.en.pdf" {
			t.Fatalf("files = %v, want exactly cv.de.pdf and cv.en.pdf", names)
		}
		de := readFile(t, filepath.Join(out, "cv.de.pdf"))
		if !strings.Contains(de, "Lebenslauf") || strings.Contains(de, "Curriculum vitae") {
			t.Errorf("cv.de.pdf source is not German:\n%s", de)
		}
		en := readFile(t, filepath.Join(out, "cv.en.pdf"))
		if !strings.Contains(en, "Curriculum vitae") {
			t.Errorf("cv.en.pdf source is not English:\n%s", en)
		}
	})

	t.Run("a failing typst writes nothing", func(t *testing.T) {
		t.Parallel()

		bin := fakeTypst(t, `echo "error: broken" >&2; exit 1`)
		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "pdf", "--dir", "testdata/render-pair", "--out", out, "--typst", bin}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "error: broken") {
			t.Errorf("stderr = %q, want typst's own message", stderr.String())
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Errorf("the output directory exists after a failed render: %v", err)
		}
	})

	t.Run("a missing typst fails", func(t *testing.T) {
		t.Parallel()

		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		missing := filepath.Join(t.TempDir(), "no-typst")
		code := run([]string{"render", "pdf", "--dir", "testdata/render-pair", "--out", out, "--typst", missing}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), "typst not found") {
			t.Errorf("code = %d, stderr = %q, want 1 and a typst not found message", code, stderr.String())
		}
	})

	t.Run("invalid data fails before typst runs", func(t *testing.T) {
		t.Parallel()

		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "pdf", "--dir", "testdata/invalid-pair", "--out", out, "--typst", "/nonexistent"}, &stdout, &stderr)
		if code != 1 || strings.Contains(stderr.String(), "typst not found") {
			t.Errorf("code = %d, stderr = %q, want the validation error", code, stderr.String())
		}
	})
}
