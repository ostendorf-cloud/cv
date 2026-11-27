package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/ostendorf-cloud/cv/internal/cv"
)

const suggestUsage = `cv suggest-skills prints what the repositories in projects say against what
the skills section claims, as a diff to review. It never writes to data/: the
owner edits the data file by hand.

Usage:
  cv suggest-skills [--dir data] [--github-url URL] [--github-timeout D]

A language no skill claims is a line starting with +. A skill whose evidence
names a project that no longer uses it is a line starting with -. A language
proves no skill level, so no level is suggested. The command exits zero when
it has nothing to suggest, and when GitHub cannot be reached: it is
information only. Invalid data fails it, like every command that reads data.
A suggestion is not remembered: a repeat run prints it again until a skill
claims the language.
`

// runSuggestSkills loads both data files, looks the projects up on GitHub and
// prints the suggestions. It only reads --dir.
func runSuggestSkills(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("suggest-skills", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, suggestUsage) }
	dir := fs.String("dir", "data", "directory holding cv.de.yaml and cv.en.yaml")
	gh := addGithubFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *gh.off {
		fmt.Fprintln(stderr, "cv: suggest-skills needs GitHub, --no-github is not supported")
		return 2
	}

	de, en, err := cv.LoadPair(filepath.Join(*dir, "cv.de.yaml"), filepath.Join(*dir, "cv.en.yaml"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	enr, err := gh.enrich(context.Background(), stderr, en)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if err := cv.SuggestSkills(de, en, enr).Write(stdout); err != nil {
		fmt.Fprintf(stderr, "cv: %v\n", err)
		return 1
	}
	return 0
}
