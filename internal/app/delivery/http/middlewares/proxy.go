package middlewares

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

	"go.uber.org/zap"

	"github.com/andybalholm/brotli"
)

func (m *Middlewares) Bridge(target string) http.Handler {
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: 100},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/fhir/")

		fullURL := target + path
		if r.URL.RawQuery != "" {
			fullURL += "?" + r.URL.RawQuery
		}

		bodyBytes, _ := r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte)
		if bodyBytes == nil {
			bodyBytes = []byte{}
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, fullURL, bytes.NewReader(bodyBytes))
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrCreateHTTPRequest(err))
			return
		}
		req.Header = r.Header.Clone()

		resp, err := client.Do(req)
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrSendHTTPRequest(err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrReadBody(readErr))
				return
			}

			fhirErr := exceptions.BuildNewCustomError(fmt.Errorf("%s", string(body)), resp.StatusCode, string(body), constvars.ErrDevServerProcess)
			utils.BuildErrorResponse(m.Log, w, fhirErr)
			return
		}

		// Read the entire response body for potential filtering
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrReadBody(readErr))
			return
		}

		// Determine if we should filter based on roles
		roles, _ := r.Context().Value(keyRoles).([]string)
		filteringRole := determineFilteringRole(roles)

		// Early return for non-filtered responses
		if filteringRole == "" {
			// No filtering needed - return response immediately
			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
			if _, err := w.Write(respBody); err != nil {
				m.Log.Warn("failed writing response body", zap.Error(err))
			}
			return
		}

		// Apply filtering for superadmin (and future roles)
		var filteredBody []byte
		var removedCount int
		var filtered bool
		switch filteringRole {
		case constvars.KonsulinRoleSuperadmin:
			if b, removed, err := m.filterResponseResourceAgainsRBAC(respBody, roles); err == nil {
				removedCount = removed
				if removed > 0 {
					filtered = true
				}
				filteredBody = b
			} else {
				// If filtering fails, fall back to original body
				m.Log.Warn("response filtering failed; returning original body", zap.Error(err))
				filteredBody = respBody
			}
		default:
			// This shouldn't happen given our early return, but safety first
			filteredBody = respBody
		}

		if filtered && removedCount > 0 {
			m.Log.Info("RBAC filtered response entries",
				zap.Int("removed", removedCount),
				zap.String("method", r.Method),
				zap.String("url", r.URL.RequestURI()),
				zap.Strings("roles", roles),
			)
		}

		// Copy headers, but recalculate Content-Length if body changed
		for k, v := range resp.Header {
			// We'll set Content-Length ourselves
			if strings.EqualFold(k, "Content-Length") {
				continue
			}
			w.Header()[k] = v
		}
		// Remove ETag if body has been modified to avoid inconsistencies
		if filtered {
			w.Header().Del("ETag")
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(filteredBody)))
		w.WriteHeader(resp.StatusCode)
		if _, err := w.Write(filteredBody); err != nil {
			m.Log.Warn("failed writing response body", zap.Error(err))
		}
	})
}

// determineFilteringRole returns the primary role for which response filtering should apply.
// For now, we only filter for superadmin requests. When empty string return, it means no filtering should be applied.
func determineFilteringRole(roles []string) string {
	for _, role := range roles {
		if strings.EqualFold(role, constvars.KonsulinRoleSuperadmin) {
			return constvars.KonsulinRoleSuperadmin
		}
	}
	return ""
}

// filterResponseResourceAgainsRBAC removes any Bundle.entry resources that are not allowed by RBAC.
// If the response is not a Bundle or JSON cannot be parsed, it returns the original body unchanged.
// It returns the possibly filtered body and the count of removed entries.
func (m *Middlewares) filterResponseResourceAgainsRBAC(body []byte, roles []string) ([]byte, int, error) {
	// 1) Early role-based check: filter only for superadmin (extendable later)
	shouldFilter := false
	for _, role := range roles {
		if strings.EqualFold(role, constvars.KonsulinRoleSuperadmin) {
			shouldFilter = true
			break
		}
	}
	if !shouldFilter {
		return body, 0, nil
	}

	// 2) Determine original encoding by attempting JSON first, then br, then gzip
	type originalEncoding string
	const (
		encodingIdentity originalEncoding = "identity"
		encodingBrotli   originalEncoding = "br"
		encodingGzip     originalEncoding = "gzip"
	)

	bodyForFiltering := body
	encDetected := encodingIdentity

	// Helper: try unmarshal to detect JSON quickly
	tryUnmarshal := func(b []byte, v any) error { return json.Unmarshal(b, v) }

	var envelope struct {
		ResourceType string `json:"resourceType"`
	}
	if err := tryUnmarshal(bodyForFiltering, &envelope); err != nil {
		// Try brotli
		if brReader := brotli.NewReader(bytes.NewReader(body)); brReader != nil {
			if decompressed, derr := io.ReadAll(brReader); derr == nil {
				if jerr := tryUnmarshal(decompressed, &envelope); jerr == nil {
					bodyForFiltering = decompressed
					encDetected = encodingBrotli
				} else {
					// Try gzip next
					if gr, gerr := gzip.NewReader(bytes.NewReader(body)); gerr == nil {
						decompressedGz, rerr := io.ReadAll(gr)
						_ = gr.Close()
						if rerr == nil && tryUnmarshal(decompressedGz, &envelope) == nil {
							bodyForFiltering = decompressedGz
							encDetected = encodingGzip
						} else {
							// Could not parse as JSON even after attempts; pass through
							return body, 0, nil
						}
					} else {
						// Not gzip either; pass through
						return body, 0, nil
					}
				}
			} else {
				// Brotli read failed; try gzip directly
				if gr, gerr := gzip.NewReader(bytes.NewReader(body)); gerr == nil {
					decompressedGz, rerr := io.ReadAll(gr)
					_ = gr.Close()
					if rerr == nil && tryUnmarshal(decompressedGz, &envelope) == nil {
						bodyForFiltering = decompressedGz
						encDetected = encodingGzip
					} else {
						return body, 0, nil
					}
				} else {
					return body, 0, nil
				}
			}
		}
	}

	// Only filter Bundles
	if !strings.EqualFold(envelope.ResourceType, "Bundle") {
		return body, 0, nil
	}

	// 3) Parse Bundle and filter entries by RBAC
	type entry struct {
		FullURL  string          `json:"fullUrl,omitempty"`
		Resource json.RawMessage `json:"resource"`
		Search   map[string]any  `json:"search,omitempty"`
	}
	var bundle struct {
		ResourceType string  `json:"resourceType"`
		ID           string  `json:"id,omitempty"`
		Type         string  `json:"type,omitempty"`
		Total        *int    `json:"total,omitempty"`
		Link         any     `json:"link,omitempty"`
		Entry        []entry `json:"entry"`
	}

	if err := json.Unmarshal(bodyForFiltering, &bundle); err != nil {
		return body, 0, nil // Malformed JSON; do not risk altering
	}

	removed := 0
	filtered := make([]entry, 0, len(bundle.Entry))
	for _, e := range bundle.Entry {
		var resEnv struct {
			ResourceType string `json:"resourceType"`
		}
		if err := json.Unmarshal(e.Resource, &resEnv); err != nil {
			filtered = append(filtered, e)
			continue
		}
		if resEnv.ResourceType == "" {
			filtered = append(filtered, e)
			continue
		}

		allowedForAnyRole := false
		for _, role := range roles {
			if allowed(m.Enforcer, role, resEnv.ResourceType, http.MethodGet) {
				allowedForAnyRole = true
				break
			}
		}
		if allowedForAnyRole {
			filtered = append(filtered, e)
		} else {
			removed++
		}
	}

	if removed == 0 {
		// Return original upstream body to avoid header inconsistencies
		return body, 0, nil
	}

	// 4) Re-marshal and re-encode with the original encoding
	bundle.Entry = filtered
	filteredJSON, err := json.Marshal(bundle)
	if err != nil {
		return body, 0, err
	}

	switch encDetected {
	case encodingIdentity:
		return filteredJSON, removed, nil
	case encodingBrotli:
		var buf bytes.Buffer
		bw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
		if _, err := bw.Write(filteredJSON); err != nil {
			_ = bw.Close()
			return body, 0, err
		}
		if err := bw.Close(); err != nil {
			return body, 0, err
		}
		return buf.Bytes(), removed, nil
	case encodingGzip:
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(filteredJSON); err != nil {
			_ = gw.Close()
			return body, 0, err
		}
		if err := gw.Close(); err != nil {
			return body, 0, err
		}
		return buf.Bytes(), removed, nil
	default:
		return filteredJSON, removed, nil
	}
}
