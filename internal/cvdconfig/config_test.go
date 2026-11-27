package cvdconfig

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "a-file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		env       map[string]string
		wantAddr  string
		wantAdmin string
		wantErrs  []string
	}{
		{
			name:      "defaults apply when only the data directory is set",
			env:       map[string]string{"DATA_DIR": dir},
			wantAddr:  ":8080",
			wantAdmin: ":8081",
		},
		{
			name:      "an explicit value overrides the default",
			env:       map[string]string{"ADDR": ":9090", "ADMIN_ADDR": ":9091", "DATA_DIR": dir},
			wantAddr:  ":9090",
			wantAdmin: ":9091",
		},
		{
			name:     "an invalid log level is reported",
			env:      map[string]string{"LOG_LEVEL": "not-a-level", "DATA_DIR": dir},
			wantErrs: []string{"level"},
		},
		{
			name:     "a missing data directory setting is reported",
			env:      map[string]string{},
			wantErrs: []string{"DATA_DIR: is required"},
		},
		{
			name:     "a data directory that does not exist is reported",
			env:      map[string]string{"DATA_DIR": filepath.Join(dir, "missing")},
			wantErrs: []string{"DATA_DIR", "no such file"},
		},
		{
			name:     "a data path that is a file is reported",
			env:      map[string]string{"DATA_DIR": file},
			wantErrs: []string{"is not a directory"},
		},
		{
			name:      "a pdf directory is optional and accepted when it exists",
			env:       map[string]string{"DATA_DIR": dir, "PDF_DIR": dir},
			wantAddr:  ":8080",
			wantAdmin: ":8081",
		},
		{
			name:     "a pdf directory that does not exist is reported",
			env:      map[string]string{"DATA_DIR": dir, "PDF_DIR": filepath.Join(dir, "missing")},
			wantErrs: []string{"PDF_DIR", "no such file"},
		},
		{
			name:     "a pdf path that is a file is reported",
			env:      map[string]string{"DATA_DIR": dir, "PDF_DIR": file},
			wantErrs: []string{"PDF_DIR", "is not a directory"},
		},
		{
			name:     "every problem is reported together",
			env:      map[string]string{"LOG_LEVEL": "nope", "DATA_DIR": filepath.Join(dir, "missing")},
			wantErrs: []string{"level", "DATA_DIR", "no such file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(func(key string) string { return tt.env[key] })
			if len(tt.wantErrs) > 0 {
				if err == nil {
					t.Fatal("Load() error = nil, want an error")
				}
				for _, want := range tt.wantErrs {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("Load() error = %q, want it to contain %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Addr != tt.wantAddr {
				t.Errorf("Addr = %q, want %q", cfg.Addr, tt.wantAddr)
			}
			if cfg.AdminAddr != tt.wantAdmin {
				t.Errorf("AdminAddr = %q, want %q", cfg.AdminAddr, tt.wantAdmin)
			}
			if cfg.DataDir != tt.env["DATA_DIR"] {
				t.Errorf("DataDir = %q, want %q", cfg.DataDir, tt.env["DATA_DIR"])
			}
			if cfg.PDFDir != tt.env["PDF_DIR"] {
				t.Errorf("PDFDir = %q, want %q", cfg.PDFDir, tt.env["PDF_DIR"])
			}
		})
	}
}

func TestLoadGitHub(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenFile, []byte("ghs_fixture_token_value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(dir, "empty")
	if err := os.WriteFile(emptyFile, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missingFile := filepath.Join(dir, "missing-token")

	tests := []struct {
		name      string
		env       map[string]string
		wantToken string
		wantTTL   time.Duration
		wantErrs  []string
	}{
		{
			name:    "nothing set gives no token and the documented lifetime",
			env:     map[string]string{},
			wantTTL: time.Hour,
		},
		{
			name:      "a token file is read and trimmed",
			env:       map[string]string{"GITHUB_TOKEN_FILE": tokenFile},
			wantToken: "ghs_fixture_token_value",
			wantTTL:   time.Hour,
		},
		{
			name:      "a token in the variable itself is read",
			env:       map[string]string{"GITHUB_TOKEN": "ghs_direct"},
			wantToken: "ghs_direct",
			wantTTL:   time.Hour,
		},
		{
			name:    "a lifetime in hours",
			env:     map[string]string{"GITHUB_CACHE_TTL": "6h"},
			wantTTL: 6 * time.Hour,
		},
		{
			name:    "a lifetime in days",
			env:     map[string]string{"GITHUB_CACHE_TTL": "2d"},
			wantTTL: 48 * time.Hour,
		},
		{
			name:     "a lifetime that is not a duration is reported",
			env:      map[string]string{"GITHUB_CACHE_TTL": "soon"},
			wantErrs: []string{"GITHUB_CACHE_TTL"},
		},
		{
			name:     "a lifetime below the minimum is reported",
			env:      map[string]string{"GITHUB_CACHE_TTL": "1s"},
			wantErrs: []string{"GITHUB_CACHE_TTL"},
		},
		{
			name:     "a lifetime above the maximum is reported",
			env:      map[string]string{"GITHUB_CACHE_TTL": "8d"},
			wantErrs: []string{"GITHUB_CACHE_TTL"},
		},
		{
			name:     "a token file that does not exist is reported without its path",
			env:      map[string]string{"GITHUB_TOKEN_FILE": missingFile},
			wantErrs: []string{"GITHUB_TOKEN_FILE"},
		},
		{
			name:     "an empty token file is reported",
			env:      map[string]string{"GITHUB_TOKEN_FILE": emptyFile},
			wantErrs: []string{"GITHUB_TOKEN_FILE"},
		},
		{
			name:     "a token and a token file together are reported",
			env:      map[string]string{"GITHUB_TOKEN": "ghs_direct", "GITHUB_TOKEN_FILE": tokenFile},
			wantErrs: []string{"GITHUB_TOKEN_FILE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := map[string]string{"DATA_DIR": dir}
			maps.Copy(env, tt.env)
			cfg, err := Load(func(key string) string { return env[key] })
			if len(tt.wantErrs) > 0 {
				if err == nil {
					t.Fatal("Load() error = nil, want an error")
				}
				for _, want := range tt.wantErrs {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("Load() error = %q, want it to contain %q", err, want)
					}
				}
				for _, secret := range []string{missingFile, emptyFile, tokenFile, "ghs_"} {
					if strings.Contains(err.Error(), secret) {
						t.Errorf("Load() error = %q, want no path or token in it", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.GitHubToken != tt.wantToken {
				t.Errorf("GitHubToken = %q, want %q", cfg.GitHubToken, tt.wantToken)
			}
			if cfg.GitHubCacheTTL != tt.wantTTL {
				t.Errorf("GitHubCacheTTL = %s, want %s", cfg.GitHubCacheTTL, tt.wantTTL)
			}
		})
	}
}
