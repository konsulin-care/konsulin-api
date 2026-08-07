package postfhir

import (
	"encoding/json"
	"testing"

	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/stretchr/testify/assert"
)

// newTestEntry builds a transactionRequestEntry with the given method/url.
func newTestEntry(method, url string) *transactionRequestEntry {
	return &transactionRequestEntry{
		Request: &struct {
			Method string `json:"method"`
			URL    string `json:"url"`
		}{Method: method, URL: url},
	}
}

func TestCollectPractitionerRoleIDsFromPutEntry(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	e := newTestEntry("PUT", "PractitionerRole/pr-1")
	e.Resource = json.RawMessage(`{"resourceType":"PractitionerRole","id":"pr-1"}`)
	collectPractitionerRoleIDsFromPutEntry(e, add)
	assert.Equal(t, []string{"pr-1"}, got)
}

func TestCollectPractitionerRoleIDsFromPutEntry_EmptyResource(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }
	collectPractitionerRoleIDsFromPutEntry(newTestEntry("PUT", "PractitionerRole/pr-1"), add)
	assert.Nil(t, got)
}

func TestCollectPractitionerRoleIDsFromPutEntry_ScheduleActors(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	e := newTestEntry("PUT", "Schedule/sched-1")
	e.Resource = json.RawMessage(`{"resourceType":"Schedule","id":"sched-1","actor":[{"reference":"PractitionerRole/pr-1"},{"reference":"PractitionerRole/pr-2"},{"reference":"Patient/pat-9"}]}`)
	collectPractitionerRoleIDsFromPutEntry(e, add)
	assert.Equal(t, []string{"pr-1", "pr-2"}, got)
}

func TestCollectPractitionerRoleIDsFromPatchEntry_PractitionerRole(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	e := newTestEntry("PATCH", "PractitionerRole/pr-7")
	collectPractitionerRoleIDsFromPatchEntry(e, nil, add)
	assert.Equal(t, []string{"pr-7"}, got)
}

func TestCollectPractitionerRoleIDsFromPatchEntry_ScheduleFromResponse(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	e := newTestEntry("PATCH", "Schedule/sched-1")
	resp := json.RawMessage(`{"resourceType":"Schedule","id":"sched-1","actor":[{"reference":"PractitionerRole/pr-3"}]}`)
	collectPractitionerRoleIDsFromPatchEntry(e, resp, add)
	assert.Equal(t, []string{"pr-3"}, got)
}

func TestCollectPractitionerRoleIDsFromPatchEntry_ScheduleEmptyResponse(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	e := newTestEntry("PATCH", "Schedule/sched-1")
	collectPractitionerRoleIDsFromPatchEntry(e, nil, add)
	assert.Nil(t, got)
}

func TestCollectPractitionerRoleIDsFromBundleEntry_PATCHDispatch(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	e := newTestEntry("PATCH", "PractitionerRole/pr-5")
	collectPractitionerRoleIDsFromBundleEntry(e, nil, add)
	assert.Equal(t, []string{"pr-5"}, got)
}

func TestCollectPractitionerRoleIDsFromBundleEntry_PUTDispatch(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	e := newTestEntry("PUT", "PractitionerRole/pr-6")
	e.Resource = json.RawMessage(`{"resourceType":"PractitionerRole","id":"pr-6"}`)
	collectPractitionerRoleIDsFromBundleEntry(e, nil, add)
	assert.Equal(t, []string{"pr-6"}, got)
}

func TestCollectPractitionerRoleIDsFromBundleEntry_SkipsDelete(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	e := newTestEntry("DELETE", "PractitionerRole/pr-1")
	collectPractitionerRoleIDsFromBundleEntry(e, nil, add)
	assert.Nil(t, got)
}

func TestAddScheduleActorRefs(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	addScheduleActorRefs([]byte(`{"resourceType":"Schedule","actor":[{"reference":"PractitionerRole/pr-1"},{"reference":"PractitionerRole/pr-2"}]}`), add)
	assert.Equal(t, []string{"pr-1", "pr-2"}, got)
}

func TestAddScheduleActorRefs_SkipsNonSchedule(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	addScheduleActorRefs([]byte(`{"resourceType":"Patient","id":"pat-1"}`), add)
	assert.Nil(t, got)
}

func TestAddSingleResourcePractitionerRoleIDs(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	addSingleResourcePractitionerRoleIDs(resourceTypePractitionerRole, "pr-9", "PUT", nil, nil, add)
	assert.Equal(t, []string{"pr-9"}, got)
}

func TestAddSingleResourcePractitionerRoleIDs_SchedulePATCHUsesResponseBody(t *testing.T) {
	var got []string
	add := func(ids []string) { got = append(got, ids...) }

	respBody := []byte(`{"resourceType":"Schedule","actor":[{"reference":"PractitionerRole/pr-4"}]}`)
	addSingleResourcePractitionerRoleIDs(resourceTypeSchedule, "sched-1", "PATCH", []byte(`{not schedule`), respBody, add)
	assert.Equal(t, []string{"pr-4"}, got)
}

func TestCollectPractitionerRoleIDsFromSingleResource_PractitionerRole(t *testing.T) {
	req := middlewares.PostFHIRProxyUserRequestDetail{Path: "/fhir/PractitionerRole/pr-1", Method: "PUT"}
	ids := collectPractitionerRoleIDsFromSingleResource(req, middlewares.PostFHIRProxyFHIRServerResponse{})
	assert.Equal(t, []string{"pr-1"}, ids)
}

func TestCollectPractitionerRoleIDsFromSingleResource_SchedulePUTUsesRequestBody(t *testing.T) {
	req := middlewares.PostFHIRProxyUserRequestDetail{
		Path:   "/fhir/Schedule/sched-1",
		Method: "PUT",
		Body:   []byte(`{"resourceType":"Schedule","actor":[{"reference":"PractitionerRole/pr-2"}]}`),
	}
	ids := collectPractitionerRoleIDsFromSingleResource(req, middlewares.PostFHIRProxyFHIRServerResponse{})
	assert.Equal(t, []string{"pr-2"}, ids)
}

func TestCollectPractitionerRoleIDsFromSingleResource_SkipsNonPUTPATCH(t *testing.T) {
	req := middlewares.PostFHIRProxyUserRequestDetail{Path: "/fhir/PractitionerRole/pr-1", Method: "GET"}
	ids := collectPractitionerRoleIDsFromSingleResource(req, middlewares.PostFHIRProxyFHIRServerResponse{})
	assert.Nil(t, ids)
}

func TestIsTransactionBundleType(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantType string
		want     bool
	}{
		{"valid transaction bundle", `{"resourceType":"Bundle","type":"transaction"}`, "transaction", true},
		{"valid transaction-response bundle", `{"resourceType":"Bundle","type":"transaction-response"}`, "transaction-response", true},
		{"wrong type", `{"resourceType":"Bundle","type":"searchset"}`, "transaction", false},
		{"empty body", "", "transaction", false},
		{"malformed json", `{not json`, "transaction", false},
		{"not a bundle", `{"resourceType":"Patient","type":"transaction"}`, "transaction", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTransactionBundleType([]byte(tt.body), tt.wantType))
		})
	}
}

func TestParseTransactionBundle(t *testing.T) {
	bundle, ok := parseTransactionBundle([]byte(`{"resourceType":"Bundle","type":"transaction","entry":[]}`))
	assert.True(t, ok)
	assert.NotNil(t, bundle)

	_, ok = parseTransactionBundle([]byte(`{"resourceType":"Bundle","type":"searchset"}`))
	assert.False(t, ok)
}

func TestParseTransactionResponseBundle(t *testing.T) {
	bundle, ok := parseTransactionResponseBundle([]byte(`{"resourceType":"Bundle","type":"transaction-response","entry":[]}`))
	assert.True(t, ok)
	assert.NotNil(t, bundle)

	_, ok = parseTransactionResponseBundle([]byte(`{"resourceType":"Bundle","type":"transaction"}`))
	assert.False(t, ok)
}
