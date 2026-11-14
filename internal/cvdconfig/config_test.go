package cvdconfig

import "testing"

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		env       map[string]string
		wantAddr  string
		wantAdmin string
		wantErr   bool
	}{
		{
			name:      "defaults apply when nothing is set",
			env:       map[string]string{},
			wantAddr:  ":8080",
			wantAdmin: ":8081",
		},
		{
			name:      "an explicit value overrides the default",
			env:       map[string]string{"ADDR": ":9090", "ADMIN_ADDR": ":9091"},
			wantAddr:  ":9090",
			wantAdmin: ":9091",
		},
		{
			name:    "an invalid log level is reported",
			env:     map[string]string{"LOG_LEVEL": "not-a-level"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(func(key string) string { return tt.env[key] })
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() error = nil, want an error")
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
		})
	}
}
