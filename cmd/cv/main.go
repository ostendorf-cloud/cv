// Command cv renders Simon Ostendorf's curriculum vitae from one source of
// data into the forms the rest of this organisation's sites and profiles
// consume. This binary holds flag parsing and command dispatch only; see
// AGENTS.md's "Scope of this milestone" for what it does not do yet.
package main

import (
	"fmt"
	"io"
	"os"
)

// Version is set at build time from the release tag.
var Version = "dev"

const usage = `cv renders Simon Ostendorf's curriculum vitae from one source of data.

Usage:
  cv <command> [flags]

Commands:
  version   print the version

Every command prints its result on stdout and everything else on stderr.
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
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "cv: unknown command %q\n%s", args[0], usage)
		return 2
	}
}
