package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ostendorf-cloud/cv/internal/cv"
)

// readmeLanguage is the language of the profile README block. The profile is
// read internationally, so the block is always English.
const readmeLanguage = "en"

// runRenderReadme implements "cv render readme". It validates both data files
// first. Without --into it writes the block on stdout. With --into it reads
// that README and writes it on stdout with the block replaced, so the caller
// decides what to do with the result and nothing here writes a file.
func runRenderReadme(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("render readme", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, renderUsage) }
	dir := fs.String("dir", "data", "directory holding cv.de.yaml and cv.en.yaml")
	into := fs.String("into", "", "README to replace the block in, written on stdout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	_, en, err := cv.LoadPair(filepath.Join(*dir, "cv.de.yaml"), filepath.Join(*dir, "cv.en.yaml"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	block, err := cv.RenderReadme(en, readmeLanguage)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	out := block
	if *into != "" {
		readme, err := os.ReadFile(*into)
		if err != nil {
			fmt.Fprintf(stderr, "cv: %v\n", err)
			return 1
		}
		if out, err = cv.ReplaceReadmeBlock(readme, block); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "cv: %v\n", err)
		return 1
	}
	return 0
}
