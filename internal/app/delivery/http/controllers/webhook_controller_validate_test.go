package controllers

import (
	"encoding/json"
	"errors"
	"testing"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateJSONBody covers the JSON acceptance gate for the async enqueue
// route (POST /{prefix}/hook/{service}): any valid JSON — object, null,
// scalar, or array — must pass so the request is forwarded as-is, while
// malformed JSON must be rejected with a 400 WEBHOOK_INVALID_JSON error.
func TestValidateJSONBody(t *testing.T) {
	validBodies := []string{
		`{"email":"patient@example.com","phone_number":"081234567890","chatwoot_id":"conv-1"}`,
		`{"email":"practitioner@example.com"}`,
		`{}`,
		`null`,
		`0`,
		`"hello"`,
		`true`,
		`[1,2,3]`,
	}
	for _, body := range validBodies {
		t.Run(body, func(t *testing.T) {
			require.True(t, json.Valid([]byte(body)), "test input must itself be valid JSON")
			assert.NoError(t, validateJSONBody([]byte(body)), "valid JSON must pass the gate")
		})
	}

	t.Run("malformed json is rejected", func(t *testing.T) {
		err := validateJSONBody([]byte(`not json`))
		require.Error(t, err, "malformed JSON must be rejected")

		var ce *exceptions.CustomError
		require.True(t, errors.As(err, &ce), "error must be a *CustomError")
		assert.Equal(t, constvars.StatusBadRequest, ce.StatusCode)
	})
}
