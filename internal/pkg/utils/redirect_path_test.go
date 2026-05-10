package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRedirectPath(t *testing.T) {
	t.Run("accepts safe internal path", func(t *testing.T) {
		err := ValidateRedirectPath("/journal?tab=1")
		assert.NoError(t, err)
	})

	t.Run("accepts safe internal path2", func(t *testing.T) {
		err := ValidateRedirectPath("/assessments?isDrawerOpen=true&assessmentId=someidhere")
		assert.NoError(t, err)
	})

	t.Run("rejects backslash in path", func(t *testing.T) {
		err := ValidateRedirectPath(`/\evil.com`)
		assert.Error(t, err)
	})

	t.Run("rejects control whitespace", func(t *testing.T) {
		err := ValidateRedirectPath("/\r\n//evil.com")
		assert.Error(t, err)
	})

	t.Run("rejects encoded protocol-relative like raw payload", func(t *testing.T) {
		err := ValidateRedirectPath("/%2F%2Fevil.com")
		assert.Error(t, err)
	})
}
