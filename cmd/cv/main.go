// Command cv renders Simon Ostendorf's curriculum vitae from one source of
// data into the forms the rest of this organisation's sites and profiles
// consume. This binary holds flag parsing and command dispatch only; see
// AGENTS.md's "Scope of this milestone" for what it does not do yet.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ostendorf-cloud/cv/internal/cv"
)

// Version is set at build time from the release tag.
var Version = "dev"

// defaultTypstTimeout bounds one typst compile. A compile of the real data
// takes well under a second; a minute leaves room for a slow runner and
// still stops a hung typst long before the CI job's own timeout.
const defaultTypstTimeout = time.Minute

const usage = `cv renders Simon Ostendorf's curriculum vitae from one source of data.

Usage:
  cv <command> [flags]

Commands:
  version    print the version
  validate   validate data/cv.de.yaml and data/cv.en.yaml
  render     render an artefact, json or pdf

Every command prints its result on stdout and everything else on stderr.
`

const validateUsage = `cv validate checks data/cv.de.yaml and data/cv.en.yaml against the data
model, reporting every problem found rather than only the first one, and
exits non zero on any.

Usage:
  cv validate [--dir data]
`

const renderUsage = `cv render writes one artefact from data/cv.de.yaml and data/cv.en.yaml.
It validates both files first and writes nothing when they are invalid.

Usage:
  cv render json [--dir data] [--out rendered] [--no-github] [--github-url URL]
  cv render pdf [--dir data] [--out rendered] [--typst typst] [--photo assets/portrait.jpg]
                [--typst-timeout 1m]

json writes cv.de.json and cv.en.json into --out.

Each project is enriched with its repository's languages and latest release
from GitHub. A failed or rate limited call logs a warning and the project is
rendered from its hand written fields. The token comes from GITHUB_TOKEN or the
file named by GITHUB_TOKEN_FILE and is optional. --no-github skips every call.

pdf writes cv.de.pdf and cv.en.pdf into --out. It fills a typst template and runs
"typst compile" with the binary --typst names, found on PATH by default. --photo
is the JPEG portrait in the header; a missing file fails the render, and an
empty value renders without a photo. --typst-timeout stops a compile that runs
longer and fails the render. It makes no GitHub call.
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
	case "render":
		return runRender(args[1:], stdout, stderr)
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

// renderPDF fills the typst template for lang and compiles it with typstBin,
// stopping typst after timeout. Expiry and the footer's date are judged
// against today's date in UTC. A nil photo renders the header without one.
func renderPDF(doc cv.Document, lang, typstBin string, photo []byte, timeout time.Duration) ([]byte, error) {
	source, err := cv.RenderTypst(doc, lang, cv.TypstOptions{Now: time.Now().UTC(), Photo: photo != nil})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return cv.CompilePDF(ctx, typstBin, source, photo)
}

// runRender dispatches "cv render <artefact>": json or pdf. It validates both data files first and writes nothing when they fail.
func runRender(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "json" && args[0] != "pdf") {
		fmt.Fprint(stderr, renderUsage)
		return 2
	}
	artefact := args[0]

	fs := flag.NewFlagSet("render "+artefact, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, renderUsage) }
	dir := fs.String("dir", "data", "directory holding cv.de.yaml and cv.en.yaml")
	out := fs.String("out", "rendered", "directory the "+artefact+" documents are written to")
	gh := addGithubFlags(fs)
	typst := fs.String("typst", "typst", "typst binary, pdf only")
	photoPath := fs.String("photo", "assets/portrait.jpg", "JPEG portrait for the header, pdf only; empty for none")
	typstTimeout := fs.Duration("typst-timeout", defaultTypstTimeout, "how long one typst compile may run, pdf only")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *typstTimeout <= 0 {
		fmt.Fprintf(stderr, "cv: --typst-timeout must be positive, got %s\n", *typstTimeout)
		return 2
	}

	de, en, err := cv.LoadPair(filepath.Join(*dir, "cv.de.yaml"), filepath.Join(*dir, "cv.en.yaml"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	// Only the JSON document carries GitHub data, the PDF never calls GitHub.
	var enr cv.Enrichment
	if artefact == "json" {
		enr, err = gh.enrich(context.Background(), stderr, de, en)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	var photo []byte
	if artefact == "pdf" && *photoPath != "" {
		photo, err = cv.LoadPhoto(*photoPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	// Render both before writing either, so a failure leaves no half set.
	docs := map[string]cv.Document{"de": de, "en": en}
	rendered := make(map[string][]byte, len(cv.Languages))
	name := cv.JSONFileName
	if artefact == "pdf" {
		name = cv.PDFFileName
	}
	for _, lang := range cv.Languages {
		var data []byte
		var err error
		if artefact == "pdf" {
			data, err = renderPDF(docs[lang], lang, *typst, photo, *typstTimeout)
		} else {
			data, err = cv.RenderJSONEnriched(docs[lang], lang, enr)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		rendered[lang] = data
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(stderr, "cv: %v\n", err)
		return 1
	}
	for _, lang := range cv.Languages {
		path := filepath.Join(*out, name(lang))
		if err := os.WriteFile(path, rendered[lang], 0o644); err != nil {
			fmt.Fprintf(stderr, "cv: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, path)
	}
	return 0
}
