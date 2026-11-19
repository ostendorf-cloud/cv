package cvdconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		})
	}
}
