package main

import (
	"os"
	"regexp"
	"testing"
)

// typstPin is the typst line of mise.toml.
var typstPin = regexp.MustCompile(`(?m)^typst = "([^"]+)"$`)

// lockedTypst is the typst block of mise.lock: its version line.
var lockedTypst = regexp.MustCompile(`(?m)^\[\[tools\.typst\]\]\nversion = "([^"]+)"$`)

// TestTypstLockMatchesMisePin keeps the typst download verified. mise checks
// the archive against the checksum in mise.lock only while the locked version
// equals the pin in mise.toml; a bump of the pin alone would install an
// unverified archive. Every platform CI runs on needs its own entry, or mise
// installs that platform unverified as well.
func TestTypstLockMatchesMisePin(t *testing.T) {
	t.Parallel()

	toml, err := os.ReadFile("../../mise.toml")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := os.ReadFile("../../mise.lock")
	if err != nil {
		t.Fatal(err)
	}

	pin := typstPin.FindSubmatch(toml)
	if pin == nil {
		t.Fatal("mise.toml pins no typst version")
	}
	locked := lockedTypst.FindSubmatch(lock)
	if locked == nil {
		t.Fatal("mise.lock holds no typst entry")
	}
	if string(pin[1]) != string(locked[1]) {
		t.Fatalf("mise.toml pins typst %s, mise.lock locks %s: copy the sha256 digests of the new release's linux archives into mise.lock", pin[1], locked[1])
	}

	for _, platform := range []string{"linux-x64", "linux-arm64"} {
		entry := regexp.MustCompile(`(?m)^\[tools\.typst\."platforms\.` + regexp.QuoteMeta(platform) + `"\]\n` +
			`checksum = "sha256:[0-9a-f]{64}"\n` +
			`url = "https://github\.com/typst/typst/releases/download/v` + regexp.QuoteMeta(string(pin[1])) + `/typst-[^"]+\.tar\.xz"$`)
		if !entry.Match(lock) {
			t.Errorf("mise.lock has no sha256 and archive of typst %s for %s", pin[1], platform)
		}
	}
}
