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

func TestRunValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "a valid pair exits zero",
			args:     []string{"validate", "--dir", "testdata/valid-pair"},
			wantCode: 0,
		},
		{
			name:     "a pair with drift exits non zero",
			args:     []string{"validate", "--dir", "testdata/invalid-pair"},
			wantCode: 1,
		},
		{
			name:     "a missing directory exits non zero",
			args:     []string{"validate", "--dir", "testdata/does-not-exist"},
			wantCode: 1,
		},
		{
			name:     "an unknown flag exits with the usage code",
			args:     []string{"validate", "--bogus"},
			wantCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("run() code = %d, want %d, stdout = %q, stderr = %q", code, tt.wantCode, stdout.String(), stderr.String())
			}
		})
	}

	t.Run("a valid pair names both files on stdout", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		run([]string{"validate", "--dir", "testdata/valid-pair"}, &stdout, &stderr)

		if !strings.Contains(stdout.String(), "cv.de.yaml") || !strings.Contains(stdout.String(), "cv.en.yaml") {
			t.Errorf("stdout = %q, want it to name both files", stdout.String())
		}
	})

	t.Run("a pair with drift names the drifted id on stderr", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		run([]string{"validate", "--dir", "testdata/invalid-pair"}, &stdout, &stderr)

		if !strings.Contains(stderr.String(), "terraform") {
			t.Errorf("stderr = %q, want it to name the drifted id", stderr.String())
		}
	})
}
