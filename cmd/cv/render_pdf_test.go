package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// portrait is the repository's own portrait, the default of --photo.
const portrait = "../../assets/portrait.jpg"

// fakeTypst writes an executable stand-in for typst into a temporary
// directory and returns its path. The CLI calls it as
// "typst compile --font-path <dir> <in> <out>".
func fakeTypst(t *testing.T, script string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// copySource copies the source to the output, so the test can read back what
// the CLI handed over. It fails unless the bundled font sits in --font-path.
const copySource = `[ "$1" = compile ] && [ "$2" = --font-path ] && [ -f "$3/Go-Regular.ttf" ] && cp "$4" "$5"`

// copyWithPhoto is copySource that also fails unless the portrait sits next
// to the source, where the template reads it.
const copyWithPhoto = copySource + ` && [ -f "$(dirname "$4")/portrait.jpg" ]`

func renderPDFArgs(bin, out string, extra ...string) []string {
	return append([]string{"render", "pdf", "--dir", "testdata/render-pair", "--out", out, "--typst", bin}, extra...)
}

func TestRunRenderPDF(t *testing.T) {
	t.Parallel()

	t.Run("a valid pair writes one pdf per language", func(t *testing.T) {
		t.Parallel()

		bin := fakeTypst(t, copyWithPhoto)
		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		code := run(renderPDFArgs(bin, out, "--photo", portrait), &stdout, &stderr)
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
		if !strings.Contains(de, `image("portrait.jpg"`) {
			t.Errorf("cv.de.pdf source places no photo:\n%s", de)
		}
		en := readFile(t, filepath.Join(out, "cv.en.pdf"))
		if !strings.Contains(en, "Curriculum vitae") {
			t.Errorf("cv.en.pdf source is not English:\n%s", en)
		}
	})

	t.Run("an empty photo flag renders without a photo", func(t *testing.T) {
		t.Parallel()

		bin := fakeTypst(t, copySource)
		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		code := run(renderPDFArgs(bin, out, "--photo", ""), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("code = %d, stderr = %q", code, stderr.String())
		}
		if src := readFile(t, filepath.Join(out, "cv.en.pdf")); strings.Contains(src, "image(") {
			t.Errorf("source places a photo although none was given:\n%s", src)
		}
	})

	t.Run("a missing photo fails before typst runs", func(t *testing.T) {
		t.Parallel()

		out := filepath.Join(t.TempDir(), "out")
		missing := filepath.Join(t.TempDir(), "portrait.jpg")
		var stdout, stderr bytes.Buffer
		code := run(renderPDFArgs("/nonexistent", out, "--photo", missing), &stdout, &stderr)
		if code != 1 {
			t.Fatalf("code = %d, want 1", code)
		}
		if msg := stderr.String(); !strings.Contains(msg, "portrait") || !strings.Contains(msg, missing) {
			t.Errorf("stderr = %q, want a portrait error naming the path", msg)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Errorf("the output directory exists after a failed render: %v", err)
		}
	})

	t.Run("a photo that is not a JPEG fails", func(t *testing.T) {
		t.Parallel()

		png := filepath.Join(t.TempDir(), "portrait.png")
		if err := os.WriteFile(png, []byte("\x89PNG\r\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := run(renderPDFArgs("/nonexistent", filepath.Join(t.TempDir(), "out"), "--photo", png), &stdout, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), "not a JPEG") {
			t.Errorf("code = %d, stderr = %q, want 1 and a not a JPEG message", code, stderr.String())
		}
	})

	t.Run("a failing typst writes nothing", func(t *testing.T) {
		t.Parallel()

		bin := fakeTypst(t, `echo "error: broken" >&2; exit 1`)
		out := filepath.Join(t.TempDir(), "out")
		var stdout, stderr bytes.Buffer
		code := run(renderPDFArgs(bin, out, "--photo", portrait), &stdout, &stderr)
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
		code := run(renderPDFArgs(missing, out, "--photo", portrait), &stdout, &stderr)
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
