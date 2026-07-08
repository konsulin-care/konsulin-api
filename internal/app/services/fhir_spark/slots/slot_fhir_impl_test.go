package slots

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// sharedSlotServer returns an httptest.Server that mocks a Blaze FHIR Slot endpoint.
func sharedSlotServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")

		switch {

		// PostTransactionBundle: POST /
		case r.Method == http.MethodPost && r.URL.Path == "/":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeOpOutcome(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			json.NewEncoder(w).Encode(fhir_dto.FHIRBundle{
				ResourceType: "Bundle",
				ID:           "txn-resp-1",
				Type:         "transaction-response",
			})

		// FindSlotByScheduleID: GET /Slot/schedule=Schedule/{id} (Blaze path-based search)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/schedule=Schedule/"):
			id := strings.TrimPrefix(r.URL.Path, "/Slot/schedule=Schedule/")
			if id == "sched-111" {
				json.NewEncoder(w).Encode(fhir_dto.FHIRBundle{
					ResourceType: "Bundle",
					Type:         "searchset",
					Total:        2,
					Entry: []fhir_dto.Entry{
						{Resource: mustMarshalSlot(fhir_dto.Slot{ResourceType: "Slot", ID: "slot-sched-1"})},
						{Resource: mustMarshalSlot(fhir_dto.Slot{ResourceType: "Slot", ID: "slot-sched-2"})},
					},
				})
				return
			}
			writeOpOutcome(w, http.StatusNotFound, "no slots found for schedule")

		// FindSlotByID & UpdateSlot: GET/PUT /Slot/{id} (2 slashes means /Slot/{id})
		case strings.Count(r.URL.Path, "/") == 2 && r.URL.Path != "/Slot":
			id := strings.TrimPrefix(r.URL.Path, "/Slot/")

			if r.Method == http.MethodGet {
				if id == "slot-123" {
					json.NewEncoder(w).Encode(fhir_dto.Slot{
						ResourceType: "Slot",
						ID:           "slot-123",
						Status:       fhir_dto.SlotStatusFree,
					})
					return
				}
				writeOpOutcome(w, http.StatusNotFound, "Slot/"+id+" not found")
				return
			}

			if r.Method == http.MethodPut {
				var req fhir_dto.Slot
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeOpOutcome(w, http.StatusBadRequest, "invalid JSON")
					return
				}
				json.NewEncoder(w).Encode(req)
				return
			}

			writeOpOutcome(w, http.StatusMethodNotAllowed, "method not allowed")

		// CreateSlot: POST /Slot
		case r.Method == http.MethodPost && r.URL.Path == "/Slot":
			var req fhir_dto.Slot
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeOpOutcome(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			req.ID = "slot-new-1"
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(req)

		// All remaining GET: search with query params
		case r.Method == http.MethodGet && r.URL.Path == "/Slot":
			schedule := r.URL.Query().Get("schedule")
			status := r.URL.Query().Get("status")
			start := r.URL.Query().Get("start")

			// FindSlotsByScheduleWithQuery + pagination: has schedule + start + status
			if schedule != "" && start != "" && status != "" {
				// Check if this is a "next" page URL
				if strings.Contains(r.URL.RawQuery, "page=2") {
					json.NewEncoder(w).Encode(fhir_dto.FHIRBundle{
						ResourceType: "Bundle",
						Type:         "searchset",
						Total:        3,
						Entry: []fhir_dto.Entry{
							{Resource: mustMarshalSlot(fhir_dto.Slot{ResourceType: "Slot", ID: "slot-page2-1"})},
						},
					})
					return
				}
				// First page with "next" link
				baseURL := fmt.Sprintf("http://%s/Slot", r.Host)
				json.NewEncoder(w).Encode(fhir_dto.FHIRBundle{
					ResourceType: "Bundle",
					Type:         "searchset",
					Total:        3,
					Link: []fhir_dto.BundleLink{
						{Relation: "next", Url: baseURL + "?schedule=Schedule/sched-111&start=lt2025-07-01T00%3A00%3A00Z&status=free&page=2"},
					},
					Entry: []fhir_dto.Entry{
						{Resource: mustMarshalSlot(fhir_dto.Slot{ResourceType: "Slot", ID: "slot-page1-1"})},
						{Resource: mustMarshalSlot(fhir_dto.Slot{ResourceType: "Slot", ID: "slot-page1-2"})},
					},
				})
				return
			}

			// Time range search (schedule + start, no status)
			if schedule != "" && start != "" && status == "" {
				json.NewEncoder(w).Encode(fhir_dto.FHIRBundle{
					ResourceType: "Bundle",
					Type:         "searchset",
					Total:        1,
					Entry: []fhir_dto.Entry{
						{Resource: mustMarshalSlot(fhir_dto.Slot{ResourceType: "Slot", ID: "slot-time-1"})},
					},
				})
				return
			}

			// Status search (schedule + status, no start)
			if schedule != "" && status != "" && start == "" {
				json.NewEncoder(w).Encode(fhir_dto.FHIRBundle{
					ResourceType: "Bundle",
					Type:         "searchset",
					Total:        1,
					Entry: []fhir_dto.Entry{
						{Resource: mustMarshalSlot(fhir_dto.Slot{
							ResourceType: "Slot",
							ID:           "slot-by-schedule-status",
							Status:       fhir_dto.SlotStatus(status),
						})},
					},
				})
				return
			}

			// Generic fallback
			writeOpOutcome(w, http.StatusNotFound, "no matching slots")

		default:
			writeOpOutcome(w, http.StatusMethodNotAllowed,
				fmt.Sprintf("unexpected: %s %s", r.Method, r.URL.String()))
		}
	}))
}

func writeOpOutcome(w http.ResponseWriter, status int, diagnostics string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(fhir_dto.OperationOutcome{
		ResourceType: "OperationOutcome",
		Issue: []fhir_dto.Issue{
			{Severity: "error", Diagnostics: diagnostics},
		},
	})
}

func mustMarshalSlot(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// newTestSlotClient creates a fresh slotFhirClient for testing.
func newTestSlotClient(srvURL string, logger *zap.Logger) *slotFhirClient {
	if srvURL[len(srvURL)-1] != '/' {
		srvURL += "/"
	}
	return &slotFhirClient{
		ResourceClient: &base.ResourceClient{
			BaseUrl: srvURL + constvars.ResourceSlot,
			Log:     logger,
			Client:  fhir_http_client.New(logger),
		},
	}
}

func TestSlot_FindSlotByID_Success(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	slot, err := client.FindSlotByID(context.Background(), "slot-123")
	require.NoError(t, err)
	require.NotNil(t, slot)
	assert.Equal(t, "slot-123", slot.ID)
	assert.Equal(t, fhir_dto.SlotStatusFree, slot.Status)
}

func TestSlot_FindSlotByID_NotFound(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	_, err := client.FindSlotByID(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestSlot_FindSlotByScheduleID_Success(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	slots, err := client.FindSlotByScheduleID(context.Background(), "sched-111")
	require.NoError(t, err)
	assert.Len(t, slots, 2)
	assert.Equal(t, "slot-sched-1", slots[0].ID)
	assert.Equal(t, "slot-sched-2", slots[1].ID)
}

func TestSlot_FindSlotByScheduleIDAndStatus_Success(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	slots, err := client.FindSlotByScheduleIDAndStatus(context.Background(), "sched-111", "free")
	require.NoError(t, err)
	assert.Len(t, slots, 1)
	assert.Equal(t, "slot-by-schedule-status", slots[0].ID)
	assert.Equal(t, fhir_dto.SlotStatusFree, slots[0].Status)
}

func TestSlot_CreateSlot_Success(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	req := &fhir_dto.Slot{
		ResourceType: "Slot",
		Status:       fhir_dto.SlotStatusFree,
	}
	slot, err := client.CreateSlot(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, slot)
	assert.Equal(t, "slot-new-1", slot.ID)
}

func TestSlot_UpdateSlot_Success(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	req := &fhir_dto.Slot{
		ResourceType: "Slot",
		ID:           "slot-123",
		Status:       fhir_dto.SlotStatusBusy,
	}
	slot, err := client.UpdateSlot(context.Background(), "slot-123", req)
	require.NoError(t, err)
	require.NotNil(t, slot)
	assert.Equal(t, "slot-123", slot.ID)
}

func TestSlot_FindSlotByScheduleAndTimeRange_Success(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	start := time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC)
	end := time.Date(2025, 6, 1, 17, 0, 0, 0, time.UTC)

	slots, err := client.FindSlotByScheduleAndTimeRange(context.Background(), "sched-111", start, end)
	require.NoError(t, err)
	assert.Len(t, slots, 1)
	assert.Equal(t, "slot-time-1", slots[0].ID)
}

func TestSlot_FindSlotsByScheduleWithQuery_Success(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	params := contracts.SlotSearchParams{
		Start:  "lt2025-07-01T00:00:00Z",
		Status: fhir_dto.SlotStatusFree,
	}

	slots, err := client.FindSlotsByScheduleWithQuery(context.Background(), "sched-111", params)
	require.NoError(t, err)
	assert.Len(t, slots, 3)
	ids := make([]string, len(slots))
	for i, s := range slots {
		ids[i] = s.ID
	}
	assert.Contains(t, ids, "slot-page1-1")
	assert.Contains(t, ids, "slot-page1-2")
	assert.Contains(t, ids, "slot-page2-1")
}

func TestSlot_PostTransactionBundle_Success(t *testing.T) {
	server := sharedSlotServer()
	defer server.Close()

	client := newTestSlotClient(server.URL, zap.NewNop())
	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        []map[string]any{},
	}

	result, err := client.PostTransactionBundle(context.Background(), bundle)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "txn-resp-1", result.ID)
	assert.Equal(t, "Bundle", result.ResourceType)
}
