package services

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// resultFhir is a contracts.ServiceRequestFhirClient double that serves a
// seeded set of ServiceRequests from Search and records every write.
type resultFhir struct {
	stored   []fhir_dto.GetServiceRequestOutput
	searches []*fhir_dto.SearchServiceRequestInput
	updates  []*fhir_dto.UpdateServiceRequestInput
	creates  int
}

var _ contracts.ServiceRequestFhirClient = (*resultFhir)(nil)

// Search records the input and serves the seeded resources, filtered by id
// when the input carries one.
func (f *resultFhir) Search(_ context.Context, input *fhir_dto.SearchServiceRequestInput) ([]fhir_dto.GetServiceRequestOutput, error) {
	f.searches = append(f.searches, input)
	if input.ID == "" {
		return append([]fhir_dto.GetServiceRequestOutput(nil), f.stored...), nil
	}
	out := make([]fhir_dto.GetServiceRequestOutput, 0, 1)
	for _, sr := range f.stored {
		if sr.ID == input.ID {
			out = append(out, sr)
		}
	}
	return out, nil
}

// Update records the write and applies it to the seeded resource.
func (f *resultFhir) Update(_ context.Context, id string, input *fhir_dto.UpdateServiceRequestInput) (*fhir_dto.GetServiceRequestOutput, error) {
	f.updates = append(f.updates, input)
	for i := range f.stored {
		if f.stored[i].ID == id {
			f.stored[i].Status = input.Status
			f.stored[i].Note = input.Note
			return &f.stored[i], nil
		}
	}
	return nil, errors.New("result fhir double: no such ServiceRequest")
}

// CreateServiceRequest counts the create and fails it: the service-result
// flow must never create resources.
func (f *resultFhir) CreateServiceRequest(context.Context, *fhir_dto.CreateServiceRequestInput) (*fhir_dto.CreateServiceRequestOutput, error) {
	f.creates++
	return nil, errors.New("result fhir double: create not expected")
}

// GetServiceRequestByIDAndVersion is unused by the flow under test.
func (*resultFhir) GetServiceRequestByIDAndVersion(context.Context, string, string) (*fhir_dto.GetServiceRequestOutput, error) {
	return nil, errors.New("result fhir double: version read not expected")
}

// EnsureAllNecessaryGroupsExists is unused by the flow under test.
func (*resultFhir) EnsureAllNecessaryGroupsExists(context.Context) error { return nil }

// newResultTestUsecase builds a ResultUsecase over the double with a fresh
// X25519 key pair as the party's use:enc key, and returns the private key so
// the test can seal payloads the usecase can open.
func newResultTestUsecase(t *testing.T, fhir *resultFhir) (*ResultUsecase, *ecdh.PrivateKey) {
	t.Helper()
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err, "the test X25519 key must generate")
	gw := &config.AppGateway{
		PartyID:       "pty-own",
		EncPrivateKey: base64.StdEncoding.EncodeToString(key.Bytes()),
	}
	return NewResultUsecase(fhir, gw, zap.NewNop()), key
}

// seededReceipt returns the invocation receipt ServiceRequest the invoke
// flow records for one id: active, owned by the practitioner, and
// identified under urn:konsulin:webhook-id with that same value (the
// read model drops identifiers, so only the id is carried here).
func seededReceipt(id string) fhir_dto.GetServiceRequestOutput {
	return fhir_dto.GetServiceRequestOutput{
		ResourceType: constvars.ResourceServiceRequest,
		ID:           id,
		Status:       "active",
		Intent:       "order",
		Requester:    fhir_dto.Reference{Reference: "Practitioner/prac-1"},
	}
}

// TestServiceResultEventStoresOpenedResultAndCompletesRequest covers
// assertion 1: a service-result event for a known webhook id opens the
// sealed result, stores it on the matching ServiceRequest and marks the
// request completed.
func TestServiceResultEventStoresOpenedResultAndCompletesRequest(t *testing.T) {
	fhir := &resultFhir{stored: []fhir_dto.GetServiceRequestOutput{seededReceipt("wh-known")}}
	uc, key := newResultTestUsecase(t, fhir)

	payload := []byte(`{"answer":42}`)
	sealed, err := envelope.Seal(payload, "result-kid", key.PublicKey())
	require.NoError(t, err, "the test payload must seal")

	require.NoError(t, uc.HandleServiceResultEvent(context.Background(), "wh-known", []byte(sealed)),
		"a service-result event for a known webhook id must be handled")

	require.Len(t, fhir.updates, 1, "the opened result must be written with one update")
	upd := fhir.updates[0]
	assert.Equal(t, "completed", upd.Status, "the ServiceRequest must be marked completed")
	require.Len(t, upd.Note, 1, "the update must carry the appended result note")
	assert.Equal(t, string(payload), upd.Note[0].Text, "the stored note must be the opened result")
	require.Len(t, upd.Identifier, 1, "the webhook id identifier must survive the update")
	assert.Equal(t, WebhookIdentifierSystem, upd.Identifier[0].System,
		"the identifier system must stay urn:konsulin:webhook-id")
	assert.Equal(t, "wh-known", upd.Identifier[0].Value, "the identifier value must stay the webhook id")
	assert.Equal(t, "completed", fhir.stored[0].Status, "the stored ServiceRequest must read completed")
	assert.Equal(t, string(payload), fhir.stored[0].Note[0].Text,
		"the stored ServiceRequest must carry the opened result")
}

// TestGetRequestOwnerOnly covers assertion 2: the owner gets status and
// result for a request, while any other caller gets not found.
func TestGetRequestOwnerOnly(t *testing.T) {
	fhir := &resultFhir{stored: []fhir_dto.GetServiceRequestOutput{seededReceipt("sr-1")}}
	fhir.stored[0].Status = "completed"
	fhir.stored[0].Note = []fhir_dto.Annotation{{Text: `{"answer":42}`}}
	uc, _ := newResultTestUsecase(t, fhir)

	got, err := uc.GetRequest(context.Background(), "Practitioner/prac-1", "sr-1")
	require.NoError(t, err, "the owner must read its own request")
	require.NotNil(t, got, "the owner read must carry a view")
	assert.Equal(t, "completed", got.Status, "the owner must get the request status")
	assert.Equal(t, `{"answer":42}`, got.Result, "the owner must get the stored result")

	_, err = uc.GetRequest(context.Background(), "Practitioner/intruder", "sr-1")
	require.Error(t, err, "another caller must be refused")
	var ce *exceptions.CustomError
	require.True(t, errors.As(err, &ce), "the refusal must render as a custom error")
	assert.Equal(t, constvars.StatusNotFound, ce.StatusCode, "another caller must get not found")
}

// TestServiceResultEventUnknownWebhookIDAcknowledgedWithoutWrite covers
// assertion 3: a result for an unknown webhook id is acknowledged without
// any write to the FHIR store.
func TestServiceResultEventUnknownWebhookIDAcknowledgedWithoutWrite(t *testing.T) {
	fhir := &resultFhir{stored: []fhir_dto.GetServiceRequestOutput{seededReceipt("wh-known")}}
	uc, key := newResultTestUsecase(t, fhir)

	payload := []byte(`{"answer":42}`)
	sealed, err := envelope.Seal(payload, "result-kid", key.PublicKey())
	require.NoError(t, err, "the test payload must seal")

	require.NoError(t, uc.HandleServiceResultEvent(context.Background(), "wh-missing", []byte(sealed)),
		"an unknown webhook id must be acknowledged")
	assert.Empty(t, fhir.updates, "an unknown webhook id must not write")
	assert.Zero(t, fhir.creates, "an unknown webhook id must not create")
}
