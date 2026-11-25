package cv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
