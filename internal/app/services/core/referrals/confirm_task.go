package referrals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"
)

// readTask fetches the raw local Task JSON. Returns the bytes and nil, or
// the error naming the read. Never panics.
func (c *Confirmer) readTask(ctx context.Context, taskID string) ([]byte, error) {
	target := c.fhirBaseURL + "/" + resourceTask + "/" + taskID
	raw, err := c.fhir.Do(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("referrals: read task: %w", err)
	}
	return raw, nil
}

// writeTask round-trips the raw Task through a generic document so every
// intake field survives the update, applies mutate and PUTs the result
// back (FHIR PUT replaces the resource).
//
// Parameters:
//   - ctx bounds the write, taskID the Task id, raw the previously read
//     Task JSON, mutate applies the status/identity changes in place.
//
// Returns nil once the FHIR server accepts the write, or the error.
// Never panics.
func (c *Confirmer) writeTask(ctx context.Context, taskID string, raw []byte,
	mutate func(map[string]any),
) error {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("referrals: decode task for update: %w", err)
	}
	mutate(doc)
	_, err := fhir_http_client.WriteResource(fhir_http_client.WriteResourceInput[map[string]any]{
		Ctx:          ctx,
		Log:          c.log,
		Client:       c.fhir,
		Method:       http.MethodPut,
		BaseUrl:      c.fhirBaseURL + "/" + resourceTask,
		ID:           taskID,
		Resource:     &doc,
		ResourceName: resourceTask,
		IDLogKey:     "task_id",
	})
	if err != nil {
		return fmt.Errorf("referrals: write task: %w", err)
	}
	return nil
}

// isListedCandidate reports whether the confirmed patient appears among
// the Task's soft-match candidate entries. Never panics.
func isListedCandidate(inputs []incomingTaskInput, patientID string) bool {
	want := constvars.FHIRRefPrefixPatient + patientID
	for _, in := range inputs {
		if in.ValueReference.Reference == want {
			return true
		}
	}
	return false
}

// createServiceRequest materializes the local referral order for the
// confirmed patient: active order, subject = the confirmed patient, the
// incoming requester kept and the referral purpose as the note.
//
// Parameters:
//   - ctx bounds the FHIR create, task the read incoming Task,
//     patientRef the confirmed patient reference.
//
// Returns the created ServiceRequest id and nil, or "" and the write
// error. Never panics.
func (c *Confirmer) createServiceRequest(ctx context.Context, task *confirmTask,
	patientRef string,
) (string, error) {
	sr := &confirmedServiceRequest{
		ResourceType: constvars.ResourceServiceRequest,
		Status:       "active",
		Intent:       taskIntent,
		Subject:      fhir_dto.Reference{Reference: patientRef},
	}
	if task.Requester != nil {
		sr.Requester = task.Requester
	}
	if purpose := strings.TrimSpace(task.Description); purpose != "" {
		sr.Note = []fhir_dto.Annotation{{Text: purpose}}
	}
	created, err := fhir_http_client.CreateResource(ctx, c.log, c.fhir,
		c.fhirBaseURL+"/"+constvars.ResourceServiceRequest, sr,
		constvars.ResourceServiceRequest, "service_request_id")
	if err != nil || created == nil {
		if err == nil {
			err = errors.New("empty ServiceRequest response")
		}
		return "", fmt.Errorf("referrals: create service request: %w", err)
	}
	if created.ID == "" {
		return "", errors.New("referrals: created ServiceRequest carries no id")
	}
	return created.ID, nil
}
