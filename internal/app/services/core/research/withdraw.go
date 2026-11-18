package research

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
)

// Withdrawal lifecycle values (BK-DATABANK-003): a withdrawn subject and an
// inactive research Consent stop extraction and mark the record as withdrawn.
const (
	researchSubjectStatusWithdrawn = "withdrawn"
	consentStatusInactive          = "inactive"
)

// ResourceUpdater replaces a raw FHIR resource by id; *resources.ResourceClient
// satisfies it.
type ResourceUpdater interface {
	// Update PUTs resource to {resourceType}/{id} and returns the response body.
	Update(ctx context.Context, resourceType, id string, resource json.RawMessage) (json.RawMessage, error)
}

// ResourceEditor is the search + update seam withdrawal needs; satisfied by
// fhir_spark's resources.ResourceClient and by test fakes.
type ResourceEditor interface {
	ResourceSearcher
	ResourceUpdater
}

// SubjectRowDeleter removes stored research rows for one subject; satisfied
// by *sqlite.ResearchStore.
type SubjectRowDeleter interface {
	// DeleteBySubject deletes every row for subjectRef and returns how many.
	DeleteBySubject(ctx context.Context, subjectRef string) (int64, error)
}

// Withdrawer enforces immediate withdrawal (BK-DATABANK-003): the subject's
// ResearchSubject is marked withdrawn and its research Consent inactive on
// the FHIR server, and the subject's derived research rows are deleted right
// away instead of waiting for the nightly pass.
type Withdrawer struct {
	editor ResourceEditor
	store  SubjectRowDeleter
}

// NewWithdrawer constructs the withdrawal flow. editor reads and updates the
// FHIR ResearchSubject/Consent resources; store deletes the subject's derived
// rows. Returns the withdrawer.
func NewWithdrawer(editor ResourceEditor, store SubjectRowDeleter) *Withdrawer {
	return &Withdrawer{editor: editor, store: store}
}

// Withdraw withdraws patient from study (BK-DATABANK-003): it marks every
// matching ResearchSubject withdrawn, marks the patient's research Consents
// inactive, then immediately deletes the patient's derived research rows.
// patient accepts a bare FHIR id ("pat-1") or a full reference; study accepts
// a bare id ("st-1") or "ResearchStudy/st-1", and an empty study withdraws
// from every study. Missing or already-withdrawn resources are a no-op
// (idempotent re-run); the row deletion always runs once the FHIR marks
// succeeded. Returns the first search/update/delete error.
func (w *Withdrawer) Withdraw(ctx context.Context, patient, study string) error {
	patientRef := patientReference(patient)
	if err := w.markSubjectsWithdrawn(ctx, patientRef, study); err != nil {
		return err
	}
	if err := w.markConsentsInactive(ctx, patientRef); err != nil {
		return err
	}
	if _, err := w.store.DeleteBySubject(ctx, patientRef); err != nil {
		return fmt.Errorf("research: withdraw: delete rows for %q: %w", patientRef, err)
	}
	return nil
}

// markSubjectsWithdrawn searches the patient's ResearchSubjects (optionally
// scoped to study), re-checks the individual and study client-side in case
// the server ignored the filters, and PUTs status "withdrawn" on each match.
// Returns the first search or update error.
func (w *Withdrawer) markSubjectsWithdrawn(ctx context.Context, patientRef, study string) error {
	params := url.Values{"individual": {patientRef}}
	if study != "" {
		params.Set("study", study)
	}
	subjects, err := w.editor.Search(ctx, constvars.ResourceResearchSubject, params)
	if err != nil {
		return fmt.Errorf("research: withdraw: search ResearchSubject: %w", err)
	}
	for _, raw := range subjects {
		rs, err := decodeResource(raw)
		if err != nil {
			return fmt.Errorf("research: withdraw: decode ResearchSubject: %w", err)
		}
		if referenceOf(rs["individual"]) != patientRef {
			continue // server ignored the individual filter: enforce client-side
		}
		if study != "" && bareID(referenceOf(rs["study"])) != bareID(study) {
			continue
		}
		if err := w.setStatus(ctx, constvars.ResourceResearchSubject, rs, researchSubjectStatusWithdrawn); err != nil {
			return err
		}
	}
	return nil
}

// markConsentsInactive searches the patient's research Consents, re-checks the
// patient reference and the research category/scope client-side, and PUTs
// status "inactive" on each match. Non-research consents are never touched.
// Returns the first search or update error.
func (w *Withdrawer) markConsentsInactive(ctx context.Context, patientRef string) error {
	consents, err := w.editor.Search(ctx, constvars.ResourceConsent, url.Values{"patient": {patientRef}})
	if err != nil {
		return fmt.Errorf("research: withdraw: search Consent: %w", err)
	}
	for _, raw := range consents {
		consent, err := decodeResource(raw)
		if err != nil {
			return fmt.Errorf("research: withdraw: decode Consent: %w", err)
		}
		if referenceOf(consent["patient"]) != patientRef {
			continue
		}
		if !hasResearchCode(consent["category"]) && !hasResearchCode(consent["scope"]) {
			continue // only research consents belong to this flow
		}
		if err := w.setStatus(ctx, constvars.ResourceConsent, consent, consentStatusInactive); err != nil {
			return err
		}
	}
	return nil
}

// setStatus read-modify-writes status onto resource via a PUT, skipping the
// call when the resource already carries that status (idempotent re-run).
// A matched resource without an id fails closed with an error. Returns nil
// on success or the encode/update error.
func (w *Withdrawer) setStatus(ctx context.Context, resourceType string, resource map[string]any, status string) error {
	if current, _ := resource["status"].(string); current == status {
		return nil
	}
	id, _ := resource["id"].(string)
	if id == "" {
		return fmt.Errorf("research: withdraw: %s resource without id cannot be updated", resourceType)
	}
	resource["status"] = status
	body, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("research: withdraw: encode %s/%s: %w", resourceType, id, err)
	}
	if _, err := w.editor.Update(ctx, resourceType, id, body); err != nil {
		return fmt.Errorf("research: withdraw: update %s/%s: %w", resourceType, id, err)
	}
	return nil
}

// patientReference normalizes a patient id or reference to Patient/{id}.
func patientReference(patient string) string {
	if strings.Contains(patient, "/") {
		return patient
	}
	return "Patient/" + patient
}

// bareID strips a trailing "Type/" prefix so "ResearchStudy/st-1" and
// "st-1" compare equal.
func bareID(ref string) string {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}
