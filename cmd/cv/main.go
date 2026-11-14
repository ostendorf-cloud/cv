// Command cv renders Simon Ostendorf's curriculum vitae from one source of
// data into the forms the rest of this organisation's sites and profiles
// consume. This binary holds flag parsing and command dispatch only; see
// AGENTS.md's "Scope of this milestone" for what it does not do yet.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ostendorf-cloud/cv/internal/cv"
)

// Version is set at build time from the release tag.
var Version = "dev"

const usage = `cv renders Simon Ostendorf's curriculum vitae from one source of data.

Usage:
  cv <command> [flags]

Commands:
  version    print the version
  validate   validate data/cv.de.yaml and data/cv.en.yaml

Every command prints its result on stdout and everything else on stderr.
`

const validateUsage = `cv validate checks data/cv.de.yaml and data/cv.en.yaml against the data
model, reporting every problem found rather than only the first one, and
exits non zero on any.

Usage:
  cv validate [--dir data]
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "cv: unknown command %q\n%s", args[0], usage)
		return 2
	}
}

// runValidate loads and validates data/cv.de.yaml and data/cv.en.yaml (or
// the two files of the same names under --dir), printing every problem
// cv.LoadPair finds and failing the process on any of them.
func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, validateUsage) }
	dir := fs.String("dir", "data", "directory holding cv.de.yaml and cv.en.yaml")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	dePath := filepath.Join(*dir, "cv.de.yaml")
	enPath := filepath.Join(*dir, "cv.en.yaml")

	if _, _, err := cv.LoadPair(dePath, enPath); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "cv: %s and %s are valid\n", dePath, enPath)
	return 0
}
