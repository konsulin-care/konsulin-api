package fhir_http_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"konsulin-service/internal/pkg/fhir_dto"

	"go.uber.org/zap"
)

// ErrFHIRRequestFailed is a sentinel error wrapping all FHIR request failures.
var ErrFHIRRequestFailed = errors.New("fhir request failed")

// FHIRHTTPClient is a thin wrapper around http.Client that handles the common
// FHIR HTTP round-trip: request creation, Content-Type header, status code
// validation, and OperationOutcome parsing.
//
// All FHIR HTTP requests in this codebase MUST go through this client's Do
// method to ensure consistent error handling and header propagation.
type FHIRHTTPClient struct {
	client *http.Client
	logger *zap.Logger
}

// New creates a new FHIRHTTPClient with a default http.Client and the given logger.
// If logger is nil, a no-op logger is used.
func New(logger *zap.Logger) *FHIRHTTPClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FHIRHTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Do sends a FHIR HTTP request and returns the raw response body on success.
// It handles:
//   - Request creation with context propagation
//   - Content-Type: application/fhir+json header
//   - HTTP execution
//   - Status code check: any status <200 or >=300 returns an error
//   - OperationOutcome parsing: if the error response contains a valid FHIR
//     OperationOutcome, its first issue diagnostics is included in the error
//   - Body reading
//
// Callers are responsible for URL construction (no double-slash guard here).
func (c *FHIRHTTPClient) Do(ctx context.Context, method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("%w: create request: %w", ErrFHIRRequestFailed, err)
	}
	req.Header.Set("Content-Type", "application/fhir+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: send request: %w", ErrFHIRRequestFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %w", ErrFHIRRequestFailed, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: %s", ErrFHIRRequestFailed, parseFHIRError(respBody, resp.StatusCode))
	}

	return respBody, nil
}

// parseFHIRError extracts a human-readable error message from a FHIR
// OperationOutcome response, or falls back to the status code and body.
func parseFHIRError(body []byte, statusCode int) string {
	var outcome fhir_dto.OperationOutcome
	if err := json.Unmarshal(body, &outcome); err == nil && len(outcome.Issue) > 0 {
		return fmt.Sprintf("%s (status %d)", outcome.Issue[0].Diagnostics, statusCode)
	}
	// Include a truncated body in the fallback to preserve upstream error context
	maxBodyLen := 200
	bodyStr := strings.TrimSpace(string(body))
	if len(bodyStr) > maxBodyLen {
		bodyStr = bodyStr[:maxBodyLen] + "..."
	}
	if bodyStr != "" {
		return fmt.Sprintf("status %d: %s", statusCode, bodyStr)
	}
	return fmt.Sprintf("status %d", statusCode)
}
