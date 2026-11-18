package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"

	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// invokeHooks is a contracts.WebhookClient double that records the sealed
// payload of the last enqueue and serves a scripted sync reply or error.
type invokeHooks struct {
	enqueueErr error
	syncReply  []byte
	syncErr    error
	to         string
	service    string
	userRef    string
	data       []byte
	enqueued   int
	synced     int
}

var _ contracts.WebhookClient = (*invokeHooks)(nil)

func (h *invokeHooks) Enqueue(_ context.Context, to, service, userRef string, data []byte) error {
	h.enqueued++
	h.to, h.service, h.userRef, h.data = to, service, userRef, data
	return h.enqueueErr
}

func (h *invokeHooks) Sync(_ context.Context, to, service, userRef string, data []byte) ([]byte, error) {
	h.synced++
	h.to, h.service, h.userRef, h.data = to, service, userRef, data
	if h.syncErr != nil {
		return nil, h.syncErr
	}
	return h.syncReply, nil
}

func (*invokeHooks) Status(context.Context, string) (string, error) { return "", nil }

// invokeRecorder is a ServiceRequestRecorder double that records the last
// receipt and returns a fixed ServiceRequest id.
type invokeRecorder struct {
	id      string
	receipt *InvocationServiceRequest
	calls   int
}

var _ ServiceRequestRecorder = (*invokeRecorder)(nil)

func (r *invokeRecorder) RecordInvocation(_ context.Context, receipt *InvocationServiceRequest) (string, error) {
	r.calls++
	r.receipt = receipt
	return r.id, nil
}

// invokeDirectory is a contracts.ListingDirectory double serving one paid
// listing for the 402 enrichment lookup.
type invokeDirectory struct {
	listing *contracts.Listing
}

var _ contracts.ListingDirectory = (*invokeDirectory)(nil)

func (d *invokeDirectory) FindListing(context.Context, string, string) (*contracts.Listing, error) {
	return d.listing, nil
}

func (*invokeDirectory) ListWebhookListings(context.Context) ([]contracts.Listing, error) {
	return nil, nil
}

func (*invokeDirectory) Enlist(context.Context, contracts.EnlistRequest) (string, error) {
	return "", nil
}

// invokeCtx returns a context carrying the backend-owned members Invoke must
// seal into the invocation context.
func invokeCtx() context.Context {
	ctx := context.WithValue(context.Background(), ContextKeyUserRef, "peppered-user-ref")
	ctx = context.WithValue(ctx, ContextKeySubjectRef, "Patient/pat-9")
	ctx = context.WithValue(ctx, constvars.CONTEXT_FHIR_ROLE, []string{constvars.KonsulinRolePractitioner})
	return context.WithValue(ctx, constvars.CONTEXT_REQUEST_ID_KEY, "req-42")
}

// TestInvokeSealsClientInputAndBackendContext covers assertion 1: the sealed
// data carries the client body as input and a backend-built context, and a
// client-supplied context inside the body is discarded.
func TestInvokeSealsClientInputAndBackendContext(t *testing.T) {
	hooks := &invokeHooks{syncReply: []byte(`{"ok":true}`)}
	recorder := &invokeRecorder{id: "sr-1"}
	inv := NewInvoker(hooks, &invokeDirectory{}, recorder)

	body := []byte(`{"context":{"user_ref":"attacker","role":"superadmin","subject_ref":"Patient/evil","request_id":"spoofed"},"q":1}`)
	out, err := inv.Invoke(invokeCtx(), "Practitioner/prac-1", "prov-1", "interpret", body, InvokeModeSync)
	require.NoError(t, err)
	require.NotNil(t, out)

	var data struct {
		Input   json.RawMessage `json:"input"`
		Context struct {
			UserRef    string `json:"user_ref"`
			Role       string `json:"role"`
			SubjectRef string `json:"subject_ref"`
			RequestID  string `json:"request_id"`
		} `json:"context"`
	}
	require.NoError(t, json.Unmarshal(hooks.data, &data))

	assert.JSONEq(t, string(body), string(data.Input), "the client body must be carried verbatim as input")
	assert.Equal(t, "peppered-user-ref", data.Context.UserRef, "user_ref must be backend-built")
	assert.Equal(t, constvars.KonsulinRolePractitioner, data.Context.Role, "role must be backend-built")
	assert.Equal(t, "Patient/pat-9", data.Context.SubjectRef, "subject_ref must be backend-built")
	assert.Equal(t, "req-42", data.Context.RequestID, "request_id must be backend-built")
	assert.Equal(t, "peppered-user-ref", hooks.userRef, "the enqueue must carry the backend user_ref")
	assert.Equal(t, "prov-1", hooks.to, "the invocation must target the provider")
	assert.Equal(t, "interpret", hooks.service, "the invocation must target the listing name")
}

// TestInvokeAsyncRecordsServiceRequestReceipt covers assertion 2: an async
// invocation creates a ServiceRequest identified by the webhook id with the
// caller as requester and returns its id.
func TestInvokeAsyncRecordsServiceRequestReceipt(t *testing.T) {
	hooks := &invokeHooks{}
	recorder := &invokeRecorder{id: "sr-webhook-7"}
	inv := NewInvoker(hooks, &invokeDirectory{}, recorder)
	inv.newWebhookID = func() string { return "wh-01JTEST0000000000000000" }

	out, err := inv.Invoke(invokeCtx(), "Practitioner/prac-1", "prov-1", "interpret",
		[]byte(`{"q":1}`), "ASYNC")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "sr-webhook-7", out.ServiceRequestID, "async must return the receipt id")
	assert.Nil(t, out.Reply, "async carries no reply")
	assert.Equal(t, 1, hooks.enqueued, "async must enqueue exactly once")
	require.NotNil(t, recorder.receipt, "async must create a ServiceRequest receipt")
	require.Len(t, recorder.receipt.Identifier, 1, "the receipt must carry one identifier")
	assert.Equal(t, WebhookIdentifierSystem, recorder.receipt.Identifier[0].System,
		"the identifier system must be urn:konsulin:webhook-id")
	assert.Equal(t, "wh-01JTEST0000000000000000", recorder.receipt.Identifier[0].Value,
		"the identifier value must be the webhook id")
	assert.Equal(t, "Practitioner/prac-1", recorder.receipt.Requester, "the requester must be the caller")
}

// TestInvokeSyncReplyAndGatewayPaymentRequired covers assertion 3: a sync
// invocation returns the opened reply, and a gateway 402 surfaces as a 402
// carrying the listing id, price and price_scope.
func TestInvokeSyncReplyAndGatewayPaymentRequired(t *testing.T) {
	t.Run("sync returns the opened reply", func(t *testing.T) {
		hooks := &invokeHooks{syncReply: []byte(`{"answer":42}`)}
		recorder := &invokeRecorder{id: "sr-1"}
		inv := NewInvoker(hooks, &invokeDirectory{}, recorder)

		out, err := inv.Invoke(invokeCtx(), "Practitioner/prac-1", "prov-1", "interpret",
			[]byte(`{"q":1}`), InvokeModeSync)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, []byte(`{"answer":42}`), out.Reply, "sync must return the opened reply")
		assert.Empty(t, out.ServiceRequestID, "sync records no receipt")
		assert.Equal(t, 0, recorder.calls, "sync must not create a ServiceRequest")
	})

	t.Run("gateway 402 surfaces as 402 with listing details", func(t *testing.T) {
		hooks := &invokeHooks{enqueueErr: &wire.Error{
			Code: "GW-PAY-002", Message: "payment required", Status: http.StatusPaymentRequired,
		}}
		recorder := &invokeRecorder{id: "sr-1"}
		dir := &invokeDirectory{listing: &contracts.Listing{
			ListingID: "01HZYLISTING000001", Price: 15000, PriceScope: "user",
		}}
		inv := NewInvoker(hooks, dir, recorder)

		out, err := inv.Invoke(invokeCtx(), "Practitioner/prac-1", "prov-1", "interpret",
			[]byte(`{"q":1}`), InvokeModeAsync)
		require.Error(t, err, "a gateway 402 must surface as an error")
		assert.Nil(t, out, "a refused invocation returns no output")
		assert.Equal(t, 0, recorder.calls, "a refused invocation records no receipt")

		var pe *PaymentRequiredError
		require.True(t, errorsAs(err, &pe), "the error must be a PaymentRequiredError")
		assert.Equal(t, "01HZYLISTING000001", pe.ListingID, "the 402 must carry the listing id")
		assert.Equal(t, int64(15000), pe.Price, "the 402 must carry the price")
		assert.Equal(t, "user", pe.PriceScope, "the 402 must carry the price scope")

		var ce *exceptions.CustomError
		require.True(t, errorsAs(err, &ce), "the 402 must unwrap to a CustomError")
		assert.Equal(t, constvars.StatusPaymentRequired, ce.StatusCode, "the status must be 402")
		assert.Contains(t, ce.ClientMessage, "01HZYLISTING000001", "the message must carry the listing id")
	})
}

// errorsAs is a tiny alias keeping the assertions readable.
func errorsAs(err error, target any) bool {
	return errors.As(err, target)
}
