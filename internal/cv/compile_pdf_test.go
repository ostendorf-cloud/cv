package cv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompilePDFKeepsSecretsFromTypst(t *testing.T) {
	// Not parallel: t.Setenv changes the process environment.
	t.Setenv("GITHUB_TOKEN", "secret-token")
	t.Setenv("LANG", "C")

	// A typst that writes its environment where the PDF should go.
	bin := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nenv > \"$5\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := CompilePDF(t.Context(), bin, []byte("= x"), nil)
	if err != nil {
		t.Fatalf("CompilePDF() error = %v", err)
	}
	if strings.Contains(string(got), "GITHUB_TOKEN") || strings.Contains(string(got), "secret-token") {
		t.Errorf("typst saw the token:\n%s", got)
	}
	if !strings.Contains(string(got), "LANG=C") {
		t.Errorf("typst lost LANG:\n%s", got)
	}
}

func TestCompilePDFStopsAtTheDeadline(t *testing.T) {
	t.Parallel()

	// A typst that never finishes. exec replaces the shell, so the kill on
	// the deadline reaches the sleeping process itself.
	bin := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := CompilePDF(ctx, bin, []byte("= x"), nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CompilePDF() error = %v, want context.DeadlineExceeded", err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("CompilePDF() returned after %s, want shortly after the deadline", took)
	}
}
