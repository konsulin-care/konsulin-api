//go:build tools

// Package tools pins CI-only Go toolchains in go.mod/go.sum so workflow steps
// run them via `go tool <name>` (lock-file enforced versions) instead of
// `go install <pkg>@<version>`.
package tools

import (
	_ "golang.org/x/vuln/cmd/govulncheck" // govulncheck (SonarCloud: lock-file enforcement)
	_ "mvdan.cc/gofumpt"                  // gofumpt formatting gate
)
