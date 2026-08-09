package config

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// envKeyPattern matches env var names in .env.example lines, whether active
// (REDIS_PASSWORD=...) or commented out (# APP_PORT=3200).
var envKeyPattern = regexp.MustCompile(`^#?\s*([A-Z_][A-Z0-9_]*)=`)

// getEnvCallPattern matches utils.GetEnvString("KEY", ...) / utils.GetEnvInt(...)
// / os.Getenv("KEY") reads inside config.go.
var getEnvCallPattern = regexp.MustCompile(`(?:utils\.GetEnv\w*|os\.Getenv)\("([A-Z_][A-Z0-9_]*)"`)

// envKeysFromExample returns every variable name documented in .env.example.
func envKeysFromExample(t *testing.T) []string {
	t.Helper()
	path := filepath.Join("..", "..", "..", ".env.example")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var keys []string
	for _, line := range strings.Split(string(content), "\n") {
		if m := envKeyPattern.FindStringSubmatch(line); m != nil {
			keys = append(keys, m[1])
		}
	}
	return keys
}

// envKeysFromConfig returns every environment variable read by config.go.
func envKeysFromConfig(t *testing.T) []string {
	t.Helper()
	content, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatalf("reading config.go: %v", err)
	}
	matches := getEnvCallPattern.FindAllStringSubmatch(string(content), -1)
	keys := make([]string, 0, len(matches))
	for _, m := range matches {
		keys = append(keys, m[1])
	}
	return keys
}

// TestEnvExampleSyncedWithConfig guards .env.example against drift:
// every env key read by config.go must be documented, and every documented
// key must still be read (no stale entries that mislead contributors).
func TestEnvExampleSyncedWithConfig(t *testing.T) {
	exampleKeys := envKeysFromExample(t)
	configKeys := envKeysFromConfig(t)

	exampleSet := toSet(exampleKeys)
	configSet := toSet(configKeys)

	var missing, stale []string
	for _, k := range configKeys {
		if !exampleSet[k] {
			missing = append(missing, k)
		}
	}
	for _, k := range exampleKeys {
		if !configSet[k] {
			stale = append(stale, k)
		}
	}

	if len(missing) > 0 || len(stale) > 0 {
		sort.Strings(missing)
		sort.Strings(stale)
		t.Errorf(".env.example is out of sync with config.go\n"+
			"  in config.go but missing from .env.example (%d):\n    %s\n"+
			"  in .env.example but not read by config.go (%d):\n    %s",
			len(missing), strings.Join(missing, "\n    "),
			len(stale), strings.Join(stale, "\n    "))
	}
}

func toSet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return set
}
