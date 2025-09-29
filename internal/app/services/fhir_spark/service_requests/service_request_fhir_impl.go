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
		"actual":       true,
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
