package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleSupertokensAPIError_WritesJSON500 verifies that recipe API errors
// produce a real 500 JSON response instead of letting net/http emit the
// implicit 200 with an empty body (which breaks the SuperTokens frontend SDK's
// response.json() call).
func TestHandleSupertokensAPIError_WritesJSON500(t *testing.T) {
	rec := httptest.NewRecorder()

	handleSupertokensAPIError(errors.New("magiclink webhook returned status 500"), nil, rec)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(body, &payload))
	msg, ok := payload["message"]
	assert.True(t, ok, "response body must carry a message in the GeneralErrorResponse shape")
	assert.NotEmpty(t, msg)
}

// TestHandleSupertokensAPIError_NilErrorStillWritesResponse verifies the
// handler is defensive: even a nil error must not leave the response unwritten.
func TestHandleSupertokensAPIError_NilErrorStillWritesResponse(t *testing.T) {
	rec := httptest.NewRecorder()

	handleSupertokensAPIError(nil, nil, rec)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
}
