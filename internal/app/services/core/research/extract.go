package research

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"konsulin-service/internal/app/drivers/sqlite"
	"konsulin-service/internal/pkg/constvars"
)

// ResourceSearcher searches raw FHIR resources on the source server: the
// extraction seam satisfied by fhir_spark's ResourceClient, with test fakes.
type ResourceSearcher interface {
	// Search GETs {resourceType}?params and returns the entry resources.
	Search(ctx context.Context, resourceType string, params url.Values) ([]json.RawMessage, error)
}

// Store persists extracted research rows and watermarks (*sqlite.ResearchStore).
type Store interface {
	// Insert stores one derived research resource row.
	Insert(ctx context.Context, r sqlite.ResearchResource) error
	// Watermark reads the stored extraction watermark ("" when never set).
	Watermark(ctx context.Context, resourceType string) (string, error)
	// SetWatermark replaces the stored extraction watermark.
	SetWatermark(ctx context.Context, resourceType, watermark string) error
}

// FHIR lifecycle values gating eligibility (BK-DATABANK-002).
const (
	researchSubjectStatusOnStudy = "on-study"
	consentStatusActive          = "active"
	consentResearchCode          = "research"
	consentStateStored           = "active"
)

// defaultResourceTypes are pulled when no explicit list is given.
var defaultResourceTypes = []string{constvars.ResourceObservation, constvars.ResourceCondition}

// Extractor pulls resources changed since the watermark for eligible
// subjects, de-identifies them and stores them (BK-DATABANK-002).
type Extractor struct {
	searcher      ResourceSearcher
	store         Store
	pseudonymKey  string
	partyID       string
	resourceTypes []string
}

// NewExtractor constructs the nightly extraction engine. searcher is the
// FHIR search seam, store persists rows and the watermark, pseudonymKey is
// APP_RESEARCH_PSEUDONYM_KEY (extraction fails closed when empty), partyID
// is recorded on every row, and resourceTypes (nil selects Observation and
// Condition) lists the types to pull. Returns the extractor.
func NewExtractor(searcher ResourceSearcher, store Store, pseudonymKey, partyID string, resourceTypes []string) *Extractor {
	if len(resourceTypes) == 0 {
		resourceTypes = append([]string(nil), defaultResourceTypes...)
	}
	return &Extractor{
		searcher:      searcher,
		store:         store,
		pseudonymKey:  pseudonymKey,
		partyID:       partyID,
		resourceTypes: resourceTypes,
	}
}

// Extract runs one extraction pass (BK-DATABANK-002): it resolves the
// eligible subjects (on-study ResearchSubject AND active research Consent),
// pulls each configured resource type changed _since its watermark,
// de-identifies and stores the resources of eligible subjects only, then
// advances the watermark. Returns nil on success, ErrEmptyPseudonymKey when
// APP_RESEARCH_PSEUDONYM_KEY is empty, or the first search/store error (a
// type's watermark only advances after all of its rows are stored).
func (e *Extractor) Extract(ctx context.Context) error {
	if strings.TrimSpace(e.pseudonymKey) == "" {
		return ErrEmptyPseudonymKey
	}
	studies, err := e.eligibleSubjects(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, resourceType := range e.resourceTypes {
		if err := e.extractType(ctx, resourceType, studies, now); err != nil {
			return err
		}
	}
	return nil
}

// extractType pulls one resource type changed since its stored watermark,
// stores eligible resources and advances that type's watermark afterwards.
func (e *Extractor) extractType(ctx context.Context, resourceType string, studies map[string]string, now time.Time) error {
	watermark, err := e.store.Watermark(ctx, resourceType)
	if err != nil {
		return fmt.Errorf("research: extract: read watermark %q: %w", resourceType, err)
	}
	params := url.Values{}
	if watermark != "" {
		params.Set("_since", watermark)
	}
	resources, err := e.searcher.Search(ctx, resourceType, params)
	if err != nil {
		return fmt.Errorf("research: extract: search %q: %w", resourceType, err)
	}
	for _, raw := range resources {
		if err := e.storeResource(ctx, raw, resourceType, studies, now); err != nil {
			return err
		}
	}
	if err := e.store.SetWatermark(ctx, resourceType, now.Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("research: extract: write watermark %q: %w", resourceType, err)
	}
	return nil
}

// storeResource decodes one changed resource, keeps it only when its subject
// is eligible, de-identifies it and inserts the row; skips without error.
func (e *Extractor) storeResource(ctx context.Context, raw json.RawMessage, resourceType string, studies map[string]string, now time.Time) error {
	resource, err := decodeResource(raw)
	if err != nil {
		return fmt.Errorf("research: extract: decode %s: %w", resourceType, err)
	}
	studyRef, ok := resourceSubject(resource)
	if !ok {
		return nil
	}
	studyID, eligible := studies[studyRef]
	if !eligible {
		return nil
	}
	sourceID, _ := resource["id"].(string)
	if sourceID == "" {
		return nil // nothing to key the row on: skip (fail closed)
	}
	deidentified, err := Deidentify(resource, e.pseudonymKey)
	if err != nil {
		return fmt.Errorf("research: extract: deidentify %s/%s: %w", resourceType, sourceID, err)
	}
	payload, err := json.Marshal(deidentified)
	if err != nil {
		return fmt.Errorf("research: extract: encode %s/%s: %w", resourceType, sourceID, err)
	}
	row := sqlite.ResearchResource{
		ID:           fmt.Sprintf("%s/%s", resourceType, sourceID),
		ResourceType: resourceType,
		SourceID:     sourceID,
		PartyID:      e.partyID,
		SubjectRef:   studyRef,
		StudyID:      studyID,
		ConsentState: consentStateStored,
		Payload:      string(payload),
		CreatedAt:    now,
	}
	if err := e.store.Insert(ctx, row); err != nil {
		return fmt.Errorf("research: extract: store %s/%s: %w", resourceType, sourceID, err)
	}
	return nil
}

// eligibleSubjects resolves the subjects passing both gates (on-study
// ResearchSubject, active research Consent): Patient ref -> study ref.
func (e *Extractor) eligibleSubjects(ctx context.Context) (map[string]string, error) {
	onStudy, err := e.onStudySubjects(ctx)
	if err != nil {
		return nil, err
	}
	consented, err := e.consentingSubjects(ctx)
	if err != nil {
		return nil, err
	}
	eligible := make(map[string]string, len(onStudy))
	for ref, study := range onStudy {
		if consented[ref] {
			eligible[ref] = study
		}
	}
	return eligible, nil
}

// onStudySubjects maps Patient ref -> study ref for on-study subjects;
// status is re-checked client-side in case the server ignored the filter.
func (e *Extractor) onStudySubjects(ctx context.Context) (map[string]string, error) {
	subjects, err := e.searcher.Search(ctx, constvars.ResourceResearchSubject,
		url.Values{"status": {researchSubjectStatusOnStudy}})
	if err != nil {
		return nil, fmt.Errorf("research: extract: search ResearchSubject: %w", err)
	}
	onStudy := make(map[string]string, len(subjects))
	for _, raw := range subjects {
		rs, err := decodeResource(raw)
		if err != nil {
			return nil, fmt.Errorf("research: extract: decode ResearchSubject: %w", err)
		}
		if status, _ := rs["status"].(string); status != researchSubjectStatusOnStudy {
			continue // server ignored the status filter: enforce client-side
		}
		ref := referenceOf(rs["individual"])
		if ref == "" {
			continue
		}
		onStudy[ref] = referenceOf(rs["study"])
	}
	return onStudy, nil
}

// consentingSubjects maps the Patient refs of active research Consents;
// status and research category/scope are re-checked client-side likewise.
func (e *Extractor) consentingSubjects(ctx context.Context) (map[string]bool, error) {
	consents, err := e.searcher.Search(ctx, constvars.ResourceConsent,
		url.Values{"status": {consentStatusActive}})
	if err != nil {
		return nil, fmt.Errorf("research: extract: search Consent: %w", err)
	}
	consented := make(map[string]bool, len(consents))
	for _, raw := range consents {
		consent, err := decodeResource(raw)
		if err != nil {
			return nil, fmt.Errorf("research: extract: decode Consent: %w", err)
		}
		if status, _ := consent["status"].(string); status != consentStatusActive {
			continue
		}
		if !hasResearchCode(consent["category"]) && !hasResearchCode(consent["scope"]) {
			continue
		}
		ref := referenceOf(consent["patient"])
		if ref != "" {
			consented[ref] = true
		}
	}
	return consented, nil
}

// decodeResource unmarshals one raw FHIR resource into a generic map.
func decodeResource(raw json.RawMessage) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// resourceSubject returns the subject Patient reference from "subject" or
// "patient", and whether one is present.
func resourceSubject(resource map[string]any) (string, bool) {
	for _, key := range []string{"subject", "patient"} {
		if ref := referenceOf(resource[key]); ref != "" {
			return ref, true
		}
	}
	return "", false
}

// referenceOf extracts a FHIR Reference string; "" for anything else.
func referenceOf(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		ref, _ := t["reference"].(string)
		return ref
	default:
		return ""
	}
}

// hasResearchCode reports whether a CodeableConcept list carries the
// "research" coding code.
func hasResearchCode(v any) bool {
	list, ok := v.([]any)
	if !ok {
		return false
	}
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		codings, ok := m["coding"].([]any)
		if !ok {
			continue
		}
		for _, c := range codings {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if code, _ := cm["code"].(string); code == consentResearchCode {
				return true
			}
		}
	}
	return false
}
