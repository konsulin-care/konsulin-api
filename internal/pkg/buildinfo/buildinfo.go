// Package buildinfo exposes build-time metadata injected via ldflags.
package buildinfo

var (
	Version    = "develop"  // deployment version (develop, staging, production)
	Tag        = "0.0.1-rc" // git tag of the build
	CommitHash = "unknown"  // git commit hash, injected at compile time
)
