package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type serviceRequestClient struct {
	contracts.ServiceRequestFhirClient
	input *fhir_dto.CreateServiceRequestInput
	err   error
}

func (c *serviceRequestClient) CreateServiceRequest(_ context.Context, input *fhir_dto.CreateServiceRequestInput) (*fhir_dto.CreateServiceRequestOutput, error) {
	c.input = input
	if c.err != nil {
		return nil, c.err
	}
	return &fhir_dto.CreateServiceRequestOutput{ID: "sr1", Meta: fhir_dto.Meta{VersionId: "2"}, Subject: input.Subject}, nil
}

func TestServiceRequestStoragePreservesCallerAndPayload(t *testing.T) {
	for _, tc := range []struct{ name, resource, id, subject, uri, patient string }{
		{"patient", "Patient", "p1", "Patient/p1", "http://localhost/hook/analyze", "p1"},
		{"patient case insensitive", "patient", "p1", "Patient/p1", "http://localhost/hook/analyze", "p1"},
		{"practitioner", "Practitioner", "pr1", "Group/guest", "http://localhost/hook/report", ""},
		{"anonymous", "", "", "", "", ""},
		{"missing ID", "Practitioner", "", "Group/guest", " ", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &serviceRequestClient{}
			storage := NewServiceRequestStorage(client, zap.NewNop())
			input := &requests.CreateServiceRequestStorageInput{UID: "user", ResourceType: tc.resource, ID: tc.id, Subject: tc.subject, InstantiatesUri: tc.uri, RawBody: json.RawMessage(`{"email":"test@example.invalid","items":[1,2]}`), Occurrence: "2026-01-02T12:00:00Z"}
			output, err := storage.Create(context.Background(), input)
			require.NoError(t, err)
			require.Equal(t, &requests.CreateServiceRequestStorageOutput{ServiceRequestID: "sr1", ServiceRequestVersion: "2", PartnerTrxID: "sr1-2", Subject: tc.subject}, output)
			require.Equal(t, "ServiceRequest", client.input.ResourceType)
			require.Equal(t, "active", client.input.Status)
			require.Equal(t, "directive", client.input.Intent)
			require.Equal(t, input.Occurrence, client.input.OccurrenceDateTime)
			require.Equal(t, tc.subject, client.input.Subject.Reference)
			if tc.id != "" && tc.resource != "" {
				require.Equal(t, tc.resource+"/"+tc.id, client.input.Requester.Reference)
			} else {
				require.Nil(t, client.input.Requester)
			}
			if tc.uri == "" || tc.uri == " " {
				require.Empty(t, client.input.InstantiatesUri)
			} else {
				require.Equal(t, []string{tc.uri}, client.input.InstantiatesUri)
			}
			require.Len(t, client.input.Note, 1)
			var note requests.NoteStorage
			require.NoError(t, json.Unmarshal([]byte(client.input.Note[0].Text), &note))
			require.JSONEq(t, string(input.RawBody), string(note.RawBody))
			require.Equal(t, "user", note.UID)
			require.Equal(t, tc.patient, note.PatientID)
		})
	}
}

func TestServiceRequestStorageErrors(t *testing.T) {
	failure := errors.New("FHIR unavailable")
	client := &serviceRequestClient{err: failure}
	storage := NewServiceRequestStorage(client, zap.NewNop())
	output, err := storage.Create(context.Background(), &requests.CreateServiceRequestStorageInput{RawBody: json.RawMessage(`{}`)})
	require.ErrorIs(t, err, failure)
	require.Nil(t, output)
	client.input = nil
	output, err = storage.Create(context.Background(), &requests.CreateServiceRequestStorageInput{RawBody: json.RawMessage(`invalid`)})
	require.Error(t, err)
	require.Nil(t, output)
	require.Nil(t, client.input)
}
