package referrals

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

// deliver posts the sealed payload to the partner endpoint and records the
// outgoing Task: "failed" on a transport error or non-2xx answer,
// "requested" on a 2xx answer.
//
// Returns the recorded Task id on success, or "" and an ErrReferralDelivery
// wrapped error. Never panics.
func (s *Sender) deliver(ctx context.Context, endpoint, ticket, sealed, caller, patientID, purpose string,
	serviceRequest json.RawMessage,
) (string, error) {
	status, err := s.post(ctx, endpoint, ticket, sealed)
	if err != nil {
		s.record(ctx, taskStatusFailed, caller, patientID, purpose, serviceRequest)
		return "", fmt.Errorf("%w: %w", ErrReferralDelivery, err)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		s.record(ctx, taskStatusFailed, caller, patientID, purpose, serviceRequest)
		return "", fmt.Errorf("%w: partner answered status %d", ErrReferralDelivery, status)
	}
	taskID := s.record(ctx, taskStatusRequested, caller, patientID, purpose, serviceRequest)
	return taskID, nil
}

// post sends the sealed payload with the bearer ticket to the partner
// endpoint. Returns the partner status code, or the transport error.
func (s *Sender) post(ctx context.Context, endpoint, ticket, sealed string) (int, error) {
	body, err := json.Marshal(map[string]string{"payload": sealed})
	if err != nil {
		return 0, fmt.Errorf("encode referral body: %w", err)
	}
	target := strings.TrimRight(endpoint, "/") + referralPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build referral request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ticket)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post referral: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// record best-effort creates the outgoing FHIR Task and returns its id
// ("" when the write fails; the failure is logged so a delivered referral
// is never failed by its audit trail).
func (s *Sender) record(ctx context.Context, status, caller, patientID, purpose string,
	serviceRequest json.RawMessage,
) string {
	task := &outgoingTask{
		ResourceType: resourceTask,
		Status:       status,
		Intent:       taskIntent,
		Description:  purpose,
		For:          fhir_dto.Reference{Reference: constvars.FHIRRefPrefixPatient + patientID},
	}
	if caller = strings.TrimSpace(caller); caller != "" {
		task.Requester = &fhir_dto.Reference{Reference: caller}
	}
	var focus struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(serviceRequest, &focus); err == nil && focus.ID != "" {
		task.Focus = &fhir_dto.Reference{Reference: constvars.ResourceServiceRequest + "/" + focus.ID}
	}
	created, err := fhir_http_client.CreateResource(ctx, s.log, s.fhir,
		s.fhirBaseURL+"/"+resourceTask, task, resourceTask, "task_id")
	if err != nil || created == nil {
		s.log.Error("referrals: record outgoing task failed", zap.Error(err))
		return ""
	}
	return created.ID
}
