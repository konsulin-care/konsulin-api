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

		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			req.Header.Set("Content-Type", "application/fhir+json")
		}
		req.Header.Set("Accept", "application/fhir+json")

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

		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrReadBody(readErr))
			return
		}

		roles, _ := r.Context().Value(keyRoles).([]string)
		filteringRole := determineFilteringRole(roles)

		if filteringRole == "" {

			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
			if _, err := w.Write(respBody); err != nil {
				m.Log.Warn("failed writing response body", zap.Error(err))
			}
			return
		}

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

				m.Log.Warn("response filtering failed; returning original body", zap.Error(err))
				filteredBody = respBody
			}
		default:

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

		for k, v := range resp.Header {

			if strings.EqualFold(k, "Content-Length") {
				continue
			}
			w.Header()[k] = v
		}

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

func determineFilteringRole(roles []string) string {
	for _, role := range roles {
		if strings.EqualFold(role, constvars.KonsulinRoleSuperadmin) {
			return constvars.KonsulinRoleSuperadmin
		}
	}
	return ""
}

func (m *Middlewares) filterResponseResourceAgainsRBAC(body []byte, roles []string) ([]byte, int, error) {

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

	type originalEncoding string
	const (
		encodingIdentity originalEncoding = "identity"
		encodingBrotli   originalEncoding = "br"
		encodingGzip     originalEncoding = "gzip"
	)

	bodyForFiltering := body
	encDetected := encodingIdentity

	tryUnmarshal := func(b []byte, v any) error { return json.Unmarshal(b, v) }

	var envelope struct {
		ResourceType string `json:"resourceType"`
	}
	if err := tryUnmarshal(bodyForFiltering, &envelope); err != nil {

		if brReader := brotli.NewReader(bytes.NewReader(body)); brReader != nil {
			if decompressed, derr := io.ReadAll(brReader); derr == nil {
				if jerr := tryUnmarshal(decompressed, &envelope); jerr == nil {
					bodyForFiltering = decompressed
					encDetected = encodingBrotli
				} else {

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
			} else {

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

	if !strings.EqualFold(envelope.ResourceType, "Bundle") {
		return body, 0, nil
	}

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
		return body, 0, nil
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

			if allowed(m.Enforcer, role, http.MethodGet, "/fhir/"+resEnv.ResourceType) {
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

		return body, 0, nil
	}

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
