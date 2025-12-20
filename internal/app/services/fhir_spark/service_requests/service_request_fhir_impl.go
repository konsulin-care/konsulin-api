package service_requests

import (
	"bytes"
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
)

var (
	serviceRequestFhirClientInstance contracts.ServiceRequestFhirClient
	onceServiceRequestFhirClient     sync.Once
)

type serviceRequestFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewServiceRequestFhirClient(baseUrl string, logger *zap.Logger) contracts.ServiceRequestFhirClient {
	onceServiceRequestFhirClient.Do(func() {
		client := &serviceRequestFhirClient{
			BaseUrl: baseUrl + constvars.ResourceServiceRequest,
			Log:     logger,
		}
		serviceRequestFhirClientInstance = client
	})
	return serviceRequestFhirClientInstance
}

func (c *serviceRequestFhirClient) CreateServiceRequest(ctx context.Context, request *fhir_dto.CreateServiceRequestInput) (*fhir_dto.CreateServiceRequestOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("serviceRequestFhirClient.CreateServiceRequest called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.CreateServiceRequest error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.CreateServiceRequest error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.CreateServiceRequest error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		return nil, exceptions.ErrCreateFHIRResource(nil, constvars.ResourceServiceRequest)
	}

	serviceRequest := new(fhir_dto.CreateServiceRequestOutput)
	if err := json.NewDecoder(resp.Body).Decode(&serviceRequest); err != nil {
		c.Log.Error("serviceRequestFhirClient.CreateServiceRequest error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceServiceRequest)
	}

	c.Log.Info("serviceRequestFhirClient.CreateServiceRequest succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_request_id", serviceRequest.ID),
	)
	return serviceRequest, nil
}

func (c *serviceRequestFhirClient) GetServiceRequestByIDAndVersion(ctx context.Context, id string, version string) (*fhir_dto.GetServiceRequestOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("serviceRequestFhirClient.GetServiceRequestVersion called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("id", id),
		zap.String("version", version),
	)

	url := c.BaseUrl + "/" + id + "/_history/" + version
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.GetServiceRequestVersion error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.GetServiceRequestVersion error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		return nil, exceptions.ErrGetFHIRResource(nil, constvars.ResourceServiceRequest)
	}

	out := new(fhir_dto.GetServiceRequestOutput)
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		c.Log.Error("serviceRequestFhirClient.GetServiceRequestVersion error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceServiceRequest)
	}

	c.Log.Info("serviceRequestFhirClient.GetServiceRequestVersion succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_request_id", out.ID),
		zap.String("version", out.Meta.VersionId),
	)
	return out, nil
}

// Search queries ServiceRequest resources by search parameters and returns an array of results.
func (c *serviceRequestFhirClient) Search(ctx context.Context, input *fhir_dto.SearchServiceRequestInput) ([]fhir_dto.GetServiceRequestOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("serviceRequestFhirClient.Search called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("id", input.ID),
	)

	// Build query string
	queryParams := input.ToQueryString()
	url := c.BaseUrl + "?" + queryParams.Encode()

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.Search error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.Search error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		c.Log.Error("serviceRequestFhirClient.Search received non-OK status",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int("status_code", resp.StatusCode),
		)
		return nil, exceptions.ErrGetFHIRResource(nil, constvars.ResourceServiceRequest)
	}

	bundle := new(fhir_dto.ServiceRequestBundle)
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		c.Log.Error("serviceRequestFhirClient.Search error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceServiceRequest)
	}

	// Extract resources from bundle entries
	results := make([]fhir_dto.GetServiceRequestOutput, 0, len(bundle.Entry))
	for _, entry := range bundle.Entry {
		results = append(results, entry.Resource)
	}

	c.Log.Info("serviceRequestFhirClient.Search succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int("result_count", len(results)),
	)
	return results, nil
}

// Update performs a PUT request to update an existing ServiceRequest resource.
func (c *serviceRequestFhirClient) Update(ctx context.Context, id string, input *fhir_dto.UpdateServiceRequestInput) (*fhir_dto.GetServiceRequestOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("serviceRequestFhirClient.Update called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("id", id),
	)

	requestJSON, err := json.Marshal(input)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.Update error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	url := c.BaseUrl + "/" + id
	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut, url, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.Update error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.Update error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK && resp.StatusCode != constvars.StatusCreated {
		c.Log.Error("serviceRequestFhirClient.Update received error status",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int("status_code", resp.StatusCode),
		)
		return nil, exceptions.ErrUpdateFHIRResource(nil, constvars.ResourceServiceRequest)
	}

	out := new(fhir_dto.GetServiceRequestOutput)
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		c.Log.Error("serviceRequestFhirClient.Update error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceServiceRequest)
	}

	c.Log.Info("serviceRequestFhirClient.Update succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_request_id", out.ID),
	)
	return out, nil
}

// EnsureGroupExists checks if Group/{groupID} exists; if not, it creates it via PUT.
func (c *serviceRequestFhirClient) ensureGroupExists(ctx context.Context, groupID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("serviceRequestFhirClient.EnsureGroupExists called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("group_id", groupID),
	)

	// Derive root FHIR base from ServiceRequest BaseUrl
	root := strings.TrimSuffix(c.BaseUrl, constvars.ResourceServiceRequest)
	groupURL := root + constvars.ResourceGroup + "/" + groupID

	// GET Group/{id}
	getReq, err := http.NewRequestWithContext(ctx, constvars.MethodGet, groupURL, nil)
	if err != nil {
		c.Log.Error("EnsureGroupExists error creating GET request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrCreateHTTPRequest(err)
	}
	getReq.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	getResp, err := client.Do(getReq)
	if err != nil {
		c.Log.Error("EnsureGroupExists error sending GET",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrSendHTTPRequest(err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode == constvars.StatusOK {
		return nil
	}
	if getResp.StatusCode != constvars.StatusNotFound {
		// Unexpected status
		return exceptions.ErrGetFHIRResource(nil, constvars.ResourceGroup)
	}

	// PUT Group/{id} with minimal valid resource
	payload := map[string]interface{}{
		"resourceType": constvars.ResourceGroup,
		"id":           groupID,
		"type":         "person",
		"actual":       false,
		"name":         groupID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return exceptions.ErrCannotMarshalJSON(err)
	}
	putReq, err := http.NewRequestWithContext(ctx, constvars.MethodPut, groupURL, bytes.NewBuffer(body))
	if err != nil {
		return exceptions.ErrCreateHTTPRequest(err)
	}
	putReq.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	putResp, err := client.Do(putReq)
	if err != nil {
		return exceptions.ErrSendHTTPRequest(err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != constvars.StatusOK && putResp.StatusCode != constvars.StatusCreated {
		return exceptions.ErrCreateFHIRResource(nil, constvars.ResourceGroup)
	}
	return nil
}

// EnsureAllNecessaryGroupsExists ensures required groups exist based on constvars.DefaultGroups
func (c *serviceRequestFhirClient) EnsureAllNecessaryGroupsExists(ctx context.Context) error {
	for _, g := range constvars.DefaultGroups {
		if strings.TrimSpace(g) == "" {
			continue
		}
		if err := c.ensureGroupExists(ctx, g); err != nil {
			return err
		}
	}
	return nil
}
