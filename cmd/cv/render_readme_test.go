package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRenderReadme(t *testing.T) {
	t.Parallel()

	t.Run("the block comes from the English data and has the markers", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "readme", "--dir", "testdata/render-pair"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("code = %d, stderr = %q", code, stderr.String())
		}
		got := stdout.String()
		if !strings.HasPrefix(got, "<!-- cv:begin -->\n") || !strings.HasSuffix(got, "<!-- cv:end -->\n") {
			t.Errorf("block is not delimited:\n%s", got)
		}
		if !strings.Contains(got, "Kubernetes English") || strings.Contains(got, "Kubernetes Deutsch") {
			t.Errorf("block is not the English data:\n%s", got)
		}
		if strings.Contains(got, "contact@example.com") || strings.Contains(got, "Neovim") {
			t.Errorf("block carries a section it must leave out:\n%s", got)
		}
	})

	t.Run("into replaces the block and keeps the text around it", func(t *testing.T) {
		t.Parallel()

		readme := filepath.Join(t.TempDir(), "README.md")
		before := "hello\n\n<!-- cv:begin -->\nold\n<!-- cv:end -->\n\nbye\n"
		if err := os.WriteFile(readme, []byte(before), 0o600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "readme", "--dir", "testdata/render-pair", "--into", readme}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("code = %d, stderr = %q", code, stderr.String())
		}
		got := stdout.String()
		if !strings.HasPrefix(got, "hello\n\n<!-- cv:begin -->\n") || !strings.HasSuffix(got, "<!-- cv:end -->\n\nbye\n") {
			t.Errorf("text around the block changed:\n%s", got)
		}
		if strings.Contains(got, "old") {
			t.Errorf("the old block is still there:\n%s", got)
		}
		if readFile(t, readme) != before {
			t.Errorf("the README on disk changed, the command must only write stdout")
		}
	})

	t.Run("refusals write nothing on stdout", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		noMarker := filepath.Join(dir, "none.md")
		if err := os.WriteFile(noMarker, []byte("no markers\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		tests := []struct {
			name string
			args []string
			want int
		}{
			{"invalid data", []string{"render", "readme", "--dir", "testdata/invalid-pair"}, 1},
			{"missing marker", []string{"render", "readme", "--dir", "testdata/render-pair", "--into", noMarker}, 1},
			{"missing readme file", []string{"render", "readme", "--dir", "testdata/render-pair", "--into", filepath.Join(dir, "absent.md")}, 1},
			{"unknown flag", []string{"render", "readme", "--nope"}, 2},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var stdout, stderr bytes.Buffer
				if code := run(tt.args, &stdout, &stderr); code != tt.want {
					t.Errorf("code = %d, want %d, stderr = %q", code, tt.want, stderr.String())
				}
				if stdout.Len() != 0 {
					t.Errorf("stdout = %q, want empty", stdout.String())
				}
			})
		}
	})
}
