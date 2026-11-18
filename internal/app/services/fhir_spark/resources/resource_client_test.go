package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newFHIRStore serves an in-memory FHIR-like store backing the round-trip
// tests: POST creates with a synthetic id, GET reads, PUT overwrites, and
// GET on the type path returns a searchset bundle of stored entries.
func newFHIRStore(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	store := map[string][]byte{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/fhir+json")

		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)

		switch {
		case r.Method == http.MethodPost && len(parts) == 1:
			body := readBody(t, r)
			var res struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(body, &res))
			if res.ID == "" {
				res.ID = strings.ToLower(parts[0]) + "-1"
				var m map[string]any
				require.NoError(t, json.Unmarshal(body, &m))
				m["id"] = res.ID
				body, _ = json.Marshal(m)
			}
			store[parts[0]+"/"+res.ID] = body
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(body)

		case r.Method == http.MethodGet && len(parts) == 2:
			stored, ok := store[parts[0]+"/"+parts[1]]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","diagnostics":"not found"}]}`))
				return
			}
			_, _ = w.Write(stored)

		case r.Method == http.MethodPut && len(parts) == 2:
			body := readBody(t, r)
			store[parts[0]+"/"+parts[1]] = body
			_, _ = w.Write(body)

		case r.Method == http.MethodGet && len(parts) == 1:
			entries := make([]map[string]json.RawMessage, 0)
			for key, stored := range store {
				if strings.HasPrefix(key, parts[0]+"/") {
					entries = append(entries, map[string]json.RawMessage{"resource": stored})
				}
			}
			bundle := map[string]any{
				"resourceType": "Bundle",
				"type":         "searchset",
				"total":        len(entries),
				"entry":        entries,
			}
			_ = json.NewEncoder(w).Encode(bundle)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return httptest.NewServer(handler)
}

func readBody(t *testing.T, r *http.Request) []byte {
	t.Helper()
	body := make([]byte, 0)
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	return body
}

// TestResourceClient_RoundTrip covers Create, Read, Update and Search
// round-tripping raw resources for every resource type this client serves.
func TestResourceClient_RoundTrip(t *testing.T) {
	server := newFHIRStore(t)
	defer server.Close()

	client := New(server.URL, zap.NewNop())
	ctx := context.Background()

	resourceTypes := []string{"Task", "Endpoint", "Consent", "ResearchSubject", "ResearchStudy"}
	for _, resourceType := range resourceTypes {
		t.Run(resourceType, func(t *testing.T) {
			createBody := json.RawMessage(fmt.Sprintf(`{"resourceType":%q,"status":"active"}`, resourceType))

			created, err := client.Create(ctx, resourceType, createBody)
			require.NoError(t, err)
			var createdRes struct {
				ResourceType string `json:"resourceType"`
				ID           string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(created, &createdRes))
			assert.Equal(t, resourceType, createdRes.ResourceType)
			assert.NotEmpty(t, createdRes.ID)

			read, err := client.Read(ctx, resourceType, createdRes.ID)
			require.NoError(t, err)
			assert.JSONEq(t, string(created), string(read))

			updateBody := json.RawMessage(fmt.Sprintf(
				`{"resourceType":%q,"id":%q,"status":"completed"}`, resourceType, createdRes.ID))
			updated, err := client.Update(ctx, resourceType, createdRes.ID, updateBody)
			require.NoError(t, err)
			assert.JSONEq(t, string(updateBody), string(updated))

			readBack, err := client.Read(ctx, resourceType, createdRes.ID)
			require.NoError(t, err)
			assert.JSONEq(t, string(updateBody), string(readBack))

			found, err := client.Search(ctx, resourceType, url.Values{"status": {"completed"}})
			require.NoError(t, err)
			require.Len(t, found, 1)
			assert.JSONEq(t, string(updateBody), string(found[0]))
		})
	}
}

// TestResourceClient_Non2xx_ReturnsMappedError asserts a non-2xx FHIR
// response surfaces as a mapped exceptions.CustomError, not a raw error.
func TestResourceClient_Non2xx_ReturnsMappedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(
			`{"resourceType":"OperationOutcome","issue":[{"severity":"error","diagnostics":"blaze down"}]}`))
	}))
	defer server.Close()

	client := New(server.URL, zap.NewNop())
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"Create", func() error {
			_, err := client.Create(ctx, "Task", json.RawMessage(`{"resourceType":"Task"}`))
			return err
		}},
		{"Read", func() error {
			_, err := client.Read(ctx, "Task", "task-1")
			return err
		}},
		{"Update", func() error {
			_, err := client.Update(ctx, "Task", "task-1", json.RawMessage(`{"resourceType":"Task"}`))
			return err
		}},
		{"Search", func() error {
			_, err := client.Search(ctx, "Task", url.Values{"status": {"requested"}})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			require.Error(t, err)
			var customErr *exceptions.CustomError
			require.ErrorAs(t, err, &customErr)
			assert.Contains(t, customErr.DevMessage, "status 503")
			assert.Contains(t, customErr.DevMessage, "blaze down")
		})
	}
}
