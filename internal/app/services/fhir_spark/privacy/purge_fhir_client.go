package privacy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_http_client"
	"net/http"
	"net/url"
	"sync"

	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

var (
	purgeFhirClientInstance contracts.PurgeFhirClient
	oncePurgeFhirClient     sync.Once
)

// purgePageSize bounds each enumeration response page so memory stays flat
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

// bundlePage is the minimal slice of a FHIR bundle needed for enumeration:
// pagination links and raw entry payloads.
type bundlePage struct {
	Link []struct {
		Relation string `json:"relation"`
		URL      string `json:"url"`
	} `json:"link"`
	Entry []json.RawMessage `json:"entry"`
}

// FindActivelyOwnedResources returns ResourceRefs for every resource the
// patient actively owns and that is safe to delete. Enumeration is driven by
// the constvars.PurgeRules registry: for each rule and ownership search
// parameter it runs a paged search and follows link.relation=next until
// exhausted. Each candidate's full body (already present in the search
// response) is checked with isDeletable, so shared resources — anything
// referencing another Patient or a Practitioner — are excluded. Results are
// deduped by resourceType + id. Passively owned and shared resources are never
// returned, which makes the result directly consumable as a delete batch.
func (c *purgeFhirClient) FindActivelyOwnedResources(ctx context.Context, patientID string) ([]contracts.ResourceRef, error) {
	seen := make(map[string]struct{})
	refs := make([]contracts.ResourceRef, 0)
	patientRef := constvars.FHIRRefPrefixPatient + patientID

	for _, rule := range constvars.PurgeRules {
		for _, param := range rule.Params {
			nextURL := fmt.Sprintf("%s%s?%s=%s&_count=%d",
				c.baseFHIRURL, rule.ResourceType, param, url.QueryEscape(patientRef), c.pageSize)
			for nextURL != "" {
				respBody, err := c.Client.Do(ctx, constvars.MethodGet, nextURL, nil)
				if err != nil {
					return nil, err
				}
				var next string
				if next, err = collectDeletableRefs(respBody, patientID, seen, &refs); err != nil {
					return nil, err
				}
				nextURL = next
			}
		}
	}
	return refs, nil
}

// collectDeletableRefs parses one search page, appends the refs of deletable
// (safety-passing, unseen) entries to refs, and returns the next page URL, if
// any.
func collectDeletableRefs(respBody []byte, patientID string, seen map[string]struct{}, refs *[]contracts.ResourceRef) (string, error) {
	var page bundlePage
	if err := json.Unmarshal(respBody, &page); err != nil {
		return "", err
	}
	for _, entry := range page.Entry {
		resourceType := gjson.GetBytes(entry, "resource.resourceType").String()
		id := gjson.GetBytes(entry, "resource.id").String()
		if resourceType == "" || id == "" {
			continue
		}
		key := resourceType + "/" + id
		if _, dup := seen[key]; dup {
			continue
		}
		if !isDeletable([]byte(gjson.GetBytes(entry, "resource").Raw), patientID) {
			continue
		}
		seen[key] = struct{}{}
		*refs = append(*refs, contracts.ResourceRef{ResourceType: resourceType, ID: id})
	}
	for _, link := range page.Link {
		if link.Relation == "next" && link.URL != "" {
			return link.URL, nil
		}
	}
	return "", nil
}

// StripPatientToShell replaces the Patient resource with a PII-free shell
// carrying only resourceType, id, the preserved meta, and active:false. A
// missing or already-purged patient (404) is a no-op success, keeping repeat
// purges idempotent.
func (c *purgeFhirClient) StripPatientToShell(ctx context.Context, patientID string) error {
	respBody, err := c.Client.Do(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, patientID), nil)
	if err != nil {
		var httpErr *fhir_http_client.FHIRHTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}

	shell := map[string]any{
		"resourceType": constvars.ResourcePatient,
		"id":           patientID,
		"active":       false,
	}
	if meta := gjson.GetBytes(respBody, "meta").Raw; meta != "" {
		shell["meta"] = json.RawMessage(meta)
	}
	body, err := json.Marshal(shell)
	if err != nil {
		return err
	}

	_, err = c.Client.Do(ctx, http.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, patientID), bytes.NewReader(body))
	return err
}
