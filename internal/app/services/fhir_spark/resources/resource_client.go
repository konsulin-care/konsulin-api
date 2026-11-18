// Package resources provides a generic client for Create/Read/Update/Search
// of raw FHIR JSON resources that have no typed DTO in this codebase
// (Task, Endpoint, Consent, ResearchSubject, ResearchStudy).
// All HTTP traffic goes through fhir_http_client.FHIRHTTPClient.Do.
package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

// ResourceClient performs generic CRUD and search operations on raw FHIR
// JSON resources against a FHIR server.
type ResourceClient struct {
	BaseURL string
	Log     *zap.Logger
	Client  *fhir_http_client.FHIRHTTPClient
}

// New creates a ResourceClient bound to the given FHIR base URL.
// All operations build their URLs from BaseURL; every request is executed
// by FHIRHTTPClient.Do so error handling and headers stay consistent.
func New(baseURL string, logger *zap.Logger) *ResourceClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ResourceClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Log:     logger,
		Client:  fhir_http_client.New(logger),
	}
}

// Create POSTs the raw resource JSON to {BaseURL}/{resourceType} and returns
// the created resource as returned by the server. Failures are mapped to
// exceptions.CustomError.
func (c *ResourceClient) Create(ctx context.Context, resourceType string, resource json.RawMessage) (json.RawMessage, error) {
	respBody, err := c.Client.Do(ctx, constvars.MethodPost,
		fmt.Sprintf("%s/%s", c.BaseURL, resourceType), bytes.NewReader(resource))
	if err != nil {
		return nil, exceptions.ErrCreateFHIRResource(err, resourceType)
	}
	return respBody, nil
}

// Read GETs {BaseURL}/{resourceType}/{id} and returns the raw resource JSON.
// Failures are mapped to exceptions.CustomError.
func (c *ResourceClient) Read(ctx context.Context, resourceType, id string) (json.RawMessage, error) {
	respBody, err := c.Client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/%s/%s", c.BaseURL, resourceType, id), nil)
	if err != nil {
		return nil, exceptions.ErrGetFHIRResource(err, resourceType)
	}
	return respBody, nil
}

// Update PUTs the raw resource JSON to {BaseURL}/{resourceType}/{id} and
// returns the server's response body. Failures are mapped to
// exceptions.CustomError.
func (c *ResourceClient) Update(ctx context.Context, resourceType, id string, resource json.RawMessage) (json.RawMessage, error) {
	respBody, err := c.Client.Do(ctx, constvars.MethodPut,
		fmt.Sprintf("%s/%s/%s", c.BaseURL, resourceType, id), bytes.NewReader(resource))
	if err != nil {
		return nil, exceptions.ErrUpdateFHIRResource(err, resourceType)
	}
	return respBody, nil
}

// Search GETs {BaseURL}/{resourceType}?params and decodes the searchset
// bundle into the raw JSON of each entry resource. Empty params skip the
// query string. Failures are mapped to exceptions.CustomError.
func (c *ResourceClient) Search(ctx context.Context, resourceType string, params url.Values) ([]json.RawMessage, error) {
	searchURL := fmt.Sprintf("%s/%s", c.BaseURL, resourceType)
	if encoded := params.Encode(); encoded != "" {
		searchURL += "?" + encoded
	}
	respBody, err := c.Client.Do(ctx, constvars.MethodGet, searchURL, nil)
	if err != nil {
		return nil, exceptions.ErrGetFHIRResource(err, resourceType)
	}
	var bundle fhir_dto.FHIRBundle
	if err := json.Unmarshal(respBody, &bundle); err != nil {
		return nil, exceptions.ErrDecodeResponse(err, resourceType)
	}
	out := make([]json.RawMessage, 0, len(bundle.Entry))
	for _, entry := range bundle.Entry {
		out = append(out, entry.Resource)
	}
	return out, nil
}
