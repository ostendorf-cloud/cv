// Package cvdconfig loads and validates cvd's own environment once,
// aggregating every problem into one error.
package cvdconfig

import (
	"fmt"
	"log/slog"

	"github.com/ostendorf-cloud/govue/envconf"
	"github.com/ostendorf-cloud/govue/logging"
)

// Config is every setting cvd reads from the environment.
type Config struct {
	Addr      string
	AdminAddr string
	LogLevel  slog.Level
}

// Load reads Config from the environment through envconf, aggregating every
// problem into one error rather than stopping at the first.
func Load(getenv func(string) string) (Config, error) {
	l := envconf.New(getenv)

	cfg := Config{
		Addr: l.String("ADDR", ":8080"),
		// Separate from Addr: govue's admin listener serves health,
		// readiness and metrics here, never through a caller of the public
		// handler. Set to "" to disable it and fall back to the single
		// listener httpserver.Run would serve instead.
		AdminAddr: l.String("ADMIN_ADDR", ":8081"),
	}

	level, err := logging.Level(l.String("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, fmt.Errorf("cvdconfig: %w", err)
	}
	cfg.LogLevel = level

	if err := l.Err(); err != nil {
		return Config{}, fmt.Errorf("cvdconfig: %w", err)
	}
	return cfg, nil
}
