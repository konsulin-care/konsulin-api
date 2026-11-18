package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/joho/godotenv"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
)

// verifyOpts holds the parsed verify flags and env file path.
type verifyOpts struct {
	standalone bool
	federation bool
	offline    bool
	path       string
}

// verify checks an env file against the offline rules.
// Parameters: args are the CLI args after "verify"; stdout and stderr are sinks.
// Returns: nil when no FAIL exists, otherwise an error with the FAIL count.
func verify(args []string, stdout, stderr io.Writer) error {
	opts, err := parseVerifyArgs(args, stderr)
	if err != nil {
		return err
	}
	values, err := godotenv.Read(opts.path)
	if err != nil {
		return fmt.Errorf("verify: read env file: %w", err)
	}
	mode, how := pickMode(values, opts.standalone, opts.federation)
	if _, err := fmt.Fprintf(stdout, "mode=%s (%s)\n", modeName(mode), how); err != nil {
		return err
	}
	fails, keyFail, err := printFindings(config.VerifyEnv(values, mode), stdout)
	if err != nil {
		return err
	}
	if mode == config.VerifyFederation && !opts.offline && !keyFail {
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

// parseVerifyArgs parses the verify flags and positional args.
// Parameters: args are the CLI args after "verify"; stderr receives flag errors.
// Returns: the parsed options, or a usage error for invalid arguments.
func parseVerifyArgs(args []string, stderr io.Writer) (verifyOpts, error) {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	standalone := fs.Bool("standalone", false, "assert standalone mode")
	federation := fs.Bool("federation", false, "assert federation mode")
	offline := fs.Bool("offline", false, "skip the online check")
	files, err := parseInterleaved(fs, args)
	if err != nil {
		return verifyOpts{}, err
	}
	if len(files) > 1 {
		return verifyOpts{}, fmt.Errorf("%w: verify takes at most one FILE", errUsage)
	}
	if *standalone && *federation {
		return verifyOpts{}, fmt.Errorf("%w: --standalone and --federation are mutually exclusive", errUsage)
	}
	opts := verifyOpts{
		standalone: *standalone,
		federation: *federation,
		offline:    *offline,
		path:       ".env",
	}
	if len(files) == 1 {
		opts.path = files[0]
	}
	return opts, nil
}

// printFindings writes one line per finding and counts the FAIL results.
// Parameters: findings are the verification results; stdout is the output sink.
// Returns: the FAIL count, whether a party key FAILed, and any write error.
func printFindings(findings []config.Finding, stdout io.Writer) (fails int, keyFail bool, err error) {
	for _, f := range findings {
		if f.Level == "FAIL" {
			fails++
			if f.Var == "APP_PARTY_PRIVATE_KEY" || f.Var == "APP_PARTY_ENC_PRIVATE_KEY" {
				keyFail = true
			}
		}
		if _, err := fmt.Fprintf(stdout, "%s %s: %s\n", f.Level, f.Var, f.Message); err != nil {
			return fails, keyFail, err
		}
	}
	return fails, keyFail, nil
}

// modeName returns the display name of a verify mode.
// Parameters: mode is the verify mode.
// Returns: "federation" or "standalone".
func modeName(mode config.VerifyMode) string {
	if mode == config.VerifyFederation {
		return "federation"
	}
	return "standalone"
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
