package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/joho/godotenv"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
)

// verify checks an env file against the offline rules.
// Parameters: args are the CLI args after "verify"; stdout and stderr are sinks.
// Returns: nil when no FAIL exists, otherwise an error with the FAIL count.
//
//nolint:gocyclo,gocognit // TODO: reduce complexity below 15 (follow-up)
func verify(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	standalone := fs.Bool("standalone", false, "assert standalone mode")
	federation := fs.Bool("federation", false, "assert federation mode")
	offline := fs.Bool("offline", false, "skip the online check")
	files, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(files) > 1 {
		return fmt.Errorf("%w: verify takes at most one FILE", errUsage)
	}
	if *standalone && *federation {
		return fmt.Errorf("%w: --standalone and --federation are mutually exclusive", errUsage)
	}
	path := ".env"
	if len(files) == 1 {
		path = files[0]
	}
	values, err := godotenv.Read(path)
	if err != nil {
		return fmt.Errorf("verify: read env file: %w", err)
	}
	mode, how := pickMode(values, *standalone, *federation)
	name := "standalone"
	if mode == config.VerifyFederation {
		name = "federation"
	}
	if _, err := fmt.Fprintf(stdout, "mode=%s (%s)\n", name, how); err != nil {
		return err
	}
	fails := 0
	keyFail := false
	for _, f := range config.VerifyEnv(values, mode) {
		if f.Level == "FAIL" {
			fails++
			if f.Var == "APP_PARTY_PRIVATE_KEY" || f.Var == "APP_PARTY_ENC_PRIVATE_KEY" {
				keyFail = true
			}
		}
		if _, err := fmt.Fprintf(stdout, "%s %s: %s\n", f.Level, f.Var, f.Message); err != nil {
			return err
		}
	}
	if mode == config.VerifyFederation && !*offline && !keyFail {
		n, err := verifyOnline(values, stdout)
		if err != nil {
			return err
		}
		fails += n
	}
	if fails > 0 {
		return fmt.Errorf("verify: %d FAIL finding(s)", fails)
	}
	return nil
}

// pickMode chooses the verify mode.
// Parameters: values is the parsed env; standalone/federation are the flags.
// Returns: the mode and "explicit" or "inferred".
func pickMode(values map[string]string, standalone, federation bool) (config.VerifyMode, string) {
	switch {
	case federation:
		return config.VerifyFederation, "explicit"
	case standalone:
		return config.VerifyStandalone, "explicit"
	}
	if strings.TrimSpace(values["APP_GATEWAY_URL"]) != "" {
		return config.VerifyFederation, "inferred"
	}
	return config.VerifyStandalone, "inferred"
}

// parseInterleaved parses flags allowing positional args anywhere.
// Parameters: fs is the flag set; args are the CLI args.
// Returns: the positional args, or a usage error.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var files []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, fmt.Errorf("%w: verify: %v", errUsage, err)
		}
		if fs.NArg() == 0 {
			return files, nil
		}
		files = append(files, fs.Arg(0))
		rest = fs.Args()[1:]
	}
}
