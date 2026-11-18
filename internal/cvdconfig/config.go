// Package cvdconfig loads and validates cvd's own environment once,
// aggregating every problem into one error.
package cvdconfig

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/ostendorf-cloud/govue/envconf"
	"github.com/ostendorf-cloud/govue/logging"
)

// Config is every setting cvd reads from the environment.
type Config struct {
	Addr      string
	AdminAddr string
	LogLevel  slog.Level
	// DataDir holds cv.de.yaml and cv.en.yaml, the data cvd serves. It is a
	// path, not a secret, so it has no _FILE twin.
	DataDir string
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
		DataDir:   l.Required("DATA_DIR"),
	}

	var problems []error

	level, err := logging.Level(l.String("LOG_LEVEL", "info"))
	if err != nil {
		problems = append(problems, err)
	}
	cfg.LogLevel = level

	if err := l.Err(); err != nil {
		problems = append(problems, err)
	}

	// A missing or unreadable directory is reported here, with every other
	// problem, so a bad mount shows up on the first start. The two files in
	// it are read and validated by cvd.App.Load.
	if cfg.DataDir != "" {
		if info, err := os.Stat(cfg.DataDir); err != nil {
			problems = append(problems, fmt.Errorf("DATA_DIR: %w", err))
		} else if !info.IsDir() {
			problems = append(problems, fmt.Errorf("DATA_DIR: %s is not a directory", cfg.DataDir))
		}
	}

	if err := errors.Join(problems...); err != nil {
		return Config{}, fmt.Errorf("cvdconfig: %w", err)
	}
	return cfg, nil
}
