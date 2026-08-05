package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_http_client"

	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

var (
	purgeFhirClientInstance contracts.PurgeFhirClient
	oncePurgeFhirClient     sync.Once
)

// purgePageSize bounds each $everything response page so memory stays flat
// regardless of how many resources a patient accumulated.
const purgePageSize = 100

type purgeFhirClient struct {
	*base.ResourceClient
	pageSize    int
	baseFHIRURL string
}

// NewPurgeFhirClient returns a singleton PurgeFhirClient bound to the given
// FHIR base URL. All HTTP traffic goes through FHIRHTTPClient.Do.
func NewPurgeFhirClient(baseUrl string, logger *zap.Logger) contracts.PurgeFhirClient {
	oncePurgeFhirClient.Do(func() {
		purgeFhirClientInstance = &purgeFhirClient{
			ResourceClient: base.New(baseUrl, constvars.ResourcePatient, logger),
			pageSize:       purgePageSize,
			baseFHIRURL:    baseUrl,
		}
	})
	return purgeFhirClientInstance
}

// bundlePage is the minimal slice of a FHIR bundle needed for id-only
// enumeration: pagination links and raw entry payloads.
type bundlePage struct {
	Link  []struct {
		Relation string `json:"relation"`
		URL      string `json:"url"`
	} `json:"link"`
	Entry []json.RawMessage `json:"entry"`
}

// GetPatientEverything returns ResourceRefs (resourceType + id only) for every
// resource linked to the patient via the $everything operation, following
// link.relation=next pagination until exhausted. Resource bodies are never
// decoded — each entry is parsed with gjson for its type and id. A missing or
// already-purged patient (404) yields an empty result without error, which
// makes the purge flow idempotent.
func (c *purgeFhirClient) GetPatientEverything(ctx context.Context, patientID string) ([]contracts.ResourceRef, error) {
	refs := make([]contracts.ResourceRef, 0)
	nextURL := fmt.Sprintf("%s/%s/$everything?_count=%d", c.BaseUrl, patientID, c.pageSize)

	for nextURL != "" {
		respBody, err := c.Client.Do(ctx, constvars.MethodGet, nextURL, nil)
		if err != nil {
			var httpErr *fhir_http_client.FHIRHTTPError
			if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
				return nil, nil
			}
			return nil, err
		}

		var page bundlePage
		if err := json.Unmarshal(respBody, &page); err != nil {
			return nil, err
		}

		for _, entry := range page.Entry {
			resourceType := gjson.GetBytes(entry, "resource.resourceType").String()
			id := gjson.GetBytes(entry, "resource.id").String()
			if resourceType == "" || id == "" {
				continue
			}
			refs = append(refs, contracts.ResourceRef{ResourceType: resourceType, ID: id})
		}

		nextURL = ""
		for _, link := range page.Link {
			if link.Relation == "next" && link.URL != "" {
				nextURL = link.URL
				break
			}
		}
	}

	return refs, nil
}

// collectSearchRefs parses a FHIR search bundle and extracts resourceType + id
// from each entry, skipping entries that carry neither.
func collectSearchRefs(respBody []byte) ([]contracts.ResourceRef, error) {
	var page bundlePage
	if err := json.Unmarshal(respBody, &page); err != nil {
		return nil, err
	}
	refs := make([]contracts.ResourceRef, 0, len(page.Entry))
	for _, entry := range page.Entry {
		resourceType := gjson.GetBytes(entry, "resource.resourceType").String()
		id := gjson.GetBytes(entry, "resource.id").String()
		if resourceType == "" || id == "" {
			continue
		}
		refs = append(refs, contracts.ResourceRef{ResourceType: resourceType, ID: id})
	}
	return refs, nil
}

// DeletePatient DELETEs the Patient resource. Returns an error on failure.
func (c *purgeFhirClient) DeletePatient(ctx context.Context, patientID string) error {
	_, err := c.Client.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%s", c.BaseUrl, patientID), nil)
	return err
}

// StripPatientPII replaces the Patient resource with a PII-free shell carrying
// only its id. Used when the Patient delete itself fails: erasure is still
// honored (no name/identifier/telecom/address remain) while leaving a valid,
// id-only resource behind.
func (c *purgeFhirClient) StripPatientPII(ctx context.Context, patientID string) error {
	shell := fmt.Sprintf(`{"resourceType":"Patient","id":%q}`, patientID)
	_, err := c.Client.Do(ctx, http.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, patientID), strings.NewReader(shell))
	return err
}

// FindCommunicationRefs returns Communications referencing the patient as
// sender or recipient, combining both searches and deduping by id.
func (c *purgeFhirClient) FindCommunicationRefs(ctx context.Context, patientID string) ([]contracts.ResourceRef, error) {
	commBase := c.baseFHIRURL + constvars.ResourceCommunication
	patientRef := constvars.FHIRRefPrefixPatient + patientID

	seen := make(map[string]struct{})
	refs := make([]contracts.ResourceRef, 0)
	for _, param := range []string{"sender", "recipient"} {
		respBody, err := c.Client.Do(ctx, constvars.MethodGet,
			fmt.Sprintf("%s?%s=%s&_count=%d", commBase, param, url.QueryEscape(patientRef), c.pageSize), nil)
		if err != nil {
			return nil, err
		}
		pageRefs, err := collectSearchRefs(respBody)
		if err != nil {
			return nil, err
		}
		for _, ref := range pageRefs {
			if _, dup := seen[ref.ID]; dup {
				continue
			}
			seen[ref.ID] = struct{}{}
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

// FindQuestionnaireResponseRefsByAuthor returns QuestionnaireResponses authored
// by the patient (author=Patient/{id} search).
func (c *purgeFhirClient) FindQuestionnaireResponseRefsByAuthor(ctx context.Context, patientID string) ([]contracts.ResourceRef, error) {
	qrBase := c.baseFHIRURL + constvars.ResourceQuestionnaireResponse
	patientRef := constvars.FHIRRefPrefixPatient + patientID

	respBody, err := c.Client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?author=%s&_count=%d", qrBase, url.QueryEscape(patientRef), c.pageSize), nil)
	if err != nil {
		return nil, err
	}
	return collectSearchRefs(respBody)
}
