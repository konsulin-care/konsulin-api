// Package main implements the party CLI (BK-PARTYID-004): one binary whose
// subcommands generate and rotate party key material. Key material goes to
// standard output as paste-ready env lines only — never to files or logs.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// errUsage reports a missing or unknown subcommand.
var errUsage = errors.New("usage: party <command>")

// main dispatches os.Args to the matching subcommand and exits non-zero on
// failure. Error messages name commands only — never key material.
func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "party:", err)
		os.Exit(1)
	}
}

// run dispatches args to the subcommand implementation: keygen writes its
// paste-ready output to stdout, help prints the synopsis to stdout, and a
// missing or unknown subcommand prints usage to stderr and fails.
// Parameters: args are the CLI arguments after the program name; stdout and
// stderr are the output sinks.
// Returns: nil on success (including help), otherwise the usage or
// subcommand error. Never panics.
func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		if err := usage(stderr); err != nil {
			return err
		}
		return errUsage
	}
	switch args[0] {
	case "keygen":
		return keygen(stdout)
	case "help", "-h", "--help":
		return usage(stdout)
	default:
		if err := usage(stderr); err != nil {
			return err
		}
		return fmt.Errorf("%w: unknown subcommand %q", errUsage, args[0])
	}
}

// usage prints the CLI synopsis.
// Parameters: w is the output sink.
// Returns: nil once the synopsis is written, or the write error.
func usage(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "usage: party <command>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  keygen   generate Ed25519 + X25519 keys and print paste-ready env values"); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "  help     print this synopsis")
	return err
}
