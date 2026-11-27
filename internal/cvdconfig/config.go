// Package cvdconfig loads and validates cvd's own environment once,
// aggregating every problem into one error.
package cvdconfig

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

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
	// PDFDir holds cv.de.pdf and cv.en.pdf, rendered at build time. It is
	// optional: without it, or without a file in it, the PDF route answers
	// 503 and everything else works.
	PDFDir string
	// GitHubToken is the optional token for the GitHub API, read from the file
	// named by GITHUB_TOKEN_FILE or from GITHUB_TOKEN. Empty means anonymous
	// calls. It is a secret: never log or print Config as a whole.
	GitHubToken string
	// GitHubCacheTTL is how long cvd keeps the GitHub data of a project before
	// it asks again.
	GitHubCacheTTL time.Duration
}

// Bounds of GITHUB_CACHE_TTL. At two calls per repository, the default of one
// refresh an hour fits a list of up to thirty repositories into GitHub's
// anonymous limit of 60 calls an hour, with no room left above that.
const (
	DefaultGitHubCacheTTL = time.Hour
	minGitHubCacheTTL     = time.Minute
	maxGitHubCacheTTL     = 7 * 24 * time.Hour
)

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
		PDFDir:    l.String("PDF_DIR", ""),
		// A set but unreadable or empty token file is a problem, because the
		// operator asked for a token. Neither variable set is valid.
		GitHubToken:    l.SecretOrFile("GITHUB_TOKEN"),
		GitHubCacheTTL: l.Duration("GITHUB_CACHE_TTL", DefaultGitHubCacheTTL, minGitHubCacheTTL, maxGitHubCacheTTL),
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
		problems = append(problems, checkDir("DATA_DIR", cfg.DataDir)...)
	}
	// A set PDF_DIR must exist: a typo would otherwise turn the PDF route
	// into a silent 503.
	if cfg.PDFDir != "" {
		problems = append(problems, checkDir("PDF_DIR", cfg.PDFDir)...)
	}

	if err := errors.Join(problems...); err != nil {
		return Config{}, fmt.Errorf("cvdconfig: %w", err)
	}
	return cfg, nil
}

// checkDir reports a path that is missing, unreadable or not a directory.
func checkDir(name, path string) []error {
	info, err := os.Stat(path)
	if err != nil {
		return []error{fmt.Errorf("%s: %w", name, err)}
	}
	if !info.IsDir() {
		return []error{fmt.Errorf("%s: %s is not a directory", name, path)}
	}
	return nil
}
