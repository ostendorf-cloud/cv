package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "version prints the build version on stdout",
			args:       []string{"version"},
			wantCode:   0,
			wantStdout: "dev\n",
		},
		{
			name:       "help prints the usage on stdout",
			args:       []string{"help"},
			wantCode:   0,
			wantStdout: usage,
		},
		{
			name:       "no arguments prints the usage on stderr and fails",
			args:       nil,
			wantCode:   2,
			wantStderr: usage,
		},
		{
			name:     "an unknown command fails and names it",
			args:     []string{"bogus"},
			wantCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("run() code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantStdout != "" && stdout.String() != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && stderr.String() != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
		})
	}

	t.Run("an unknown command names it in stderr", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		run([]string{"bogus"}, &stdout, &stderr)
		if !strings.Contains(stderr.String(), "bogus") {
			t.Errorf("stderr = %q, want it to name the unknown command", stderr.String())
		}
	})
}
