package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaults verifies the build metadata defaults present when the binary is
// compiled without ldflags injection (e.g. local development, unit tests).
func TestDefaults(t *testing.T) {
	assert.Equal(t, "develop", Version, "default Version should be develop")
	assert.Equal(t, "0.0.1-rc", Tag, "default Tag should be 0.0.1-rc")
	assert.Equal(t, "unknown", CommitHash, "default CommitHash should be unknown")
}
