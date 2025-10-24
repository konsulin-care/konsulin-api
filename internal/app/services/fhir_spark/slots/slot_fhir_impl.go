package slots

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	slotFhirClientInstance contracts.SlotFhirClient
	onceSlotFhirClient     sync.Once
)

type slotFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewSlotFhirClient(baseUrl string, logger *zap.Logger) contracts.SlotFhirClient {
	onceSlotFhirClient.Do(func() {
		client := &slotFhirClient{
			BaseUrl: baseUrl + constvars.ResourceSlot,
			Log:     logger,
		}
		slotFhirClientInstance = client
	})
	return slotFhirClientInstance
}

func (c *slotFhirClient) FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.FindSlotByScheduleID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, scheduleID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/schedule=Schedule/%s", c.BaseUrl, scheduleID),
		nil,
	)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("slotFhirClient.FindSlotByScheduleID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("slotFhirClient.FindSlotByScheduleID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("slotFhirClient.FindSlotByScheduleID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	slotsFhir := make([]fhir_dto.Slot, len(result.Entry))
	for i, entry := range result.Entry {
		slotsFhir[i] = entry.Resource
	}

	c.Log.Info("slotFhirClient.FindSlotByScheduleID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingSlotsCountKey, len(slotsFhir)),
	)
	return slotsFhir, nil
}

func (c *slotFhirClient) FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.FindSlotByScheduleIDAndStatus called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, scheduleID),
		zap.String(constvars.LoggingScheduleStatusKey, status),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?schedule=Schedule/%s&status=%s", c.BaseUrl, scheduleID, status),
		nil,
	)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleIDAndStatus error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleIDAndStatus error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("slotFhirClient.FindSlotByScheduleIDAndStatus error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("slotFhirClient.FindSlotByScheduleIDAndStatus error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("slotFhirClient.FindSlotByScheduleIDAndStatus FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleIDAndStatus error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	slotsFhir := make([]fhir_dto.Slot, len(result.Entry))
	for i, entry := range result.Entry {
		slotsFhir[i] = entry.Resource
	}

	c.Log.Info("slotFhirClient.FindSlotByScheduleIDAndStatus succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingSlotsCountKey, len(slotsFhir)),
	)
	return slotsFhir, nil
}

func (c *slotFhirClient) CreateSlot(ctx context.Context, request *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.CreateSlot called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("slotFhirClient.CreateSlot error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("slotFhirClient.CreateSlot error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("slotFhirClient.CreateSlot error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("slotFhirClient.CreateSlot error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("slotFhirClient.CreateSlot error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("slotFhirClient.CreateSlot FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	slotFhir := new(fhir_dto.Slot)
	err = json.NewDecoder(resp.Body).Decode(&slotFhir)
	if err != nil {
		c.Log.Error("slotFhirClient.CreateSlot error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	c.Log.Info("slotFhirClient.CreateSlot succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSlotsIDKey, slotFhir.ID),
	)
	return slotFhir, nil
}

func (c *slotFhirClient) FindSlotByScheduleAndTimeRange(ctx context.Context, scheduleID string, startTime, endTime time.Time) ([]fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.FindSlotByScheduleAndTimeRange called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, scheduleID),
		zap.String(constvars.LoggingSlotsStartKey, startTime.Format(time.RFC3339)),
		zap.String(constvars.LoggingSlotsEndKey, endTime.Format(time.RFC3339)),
	)

	queryURL := fmt.Sprintf(
		"%s?schedule=Schedule/%s&start=eq%s&end=eq%s",
		c.BaseUrl,
		scheduleID,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
	)
	c.Log.Info("slotFhirClient.FindSlotByScheduleAndTimeRange built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("url", queryURL),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, queryURL, nil)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleAndTimeRange error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleAndTimeRange error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("slotFhirClient.FindSlotByScheduleAndTimeRange error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("slotFhirClient.FindSlotByScheduleAndTimeRange error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("slotFhirClient.FindSlotByScheduleAndTimeRange FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleAndTimeRange error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	slots := make([]fhir_dto.Slot, len(result.Entry))
	for i, entry := range result.Entry {
		slots[i] = entry.Resource
	}

	c.Log.Info("slotFhirClient.FindSlotByScheduleAndTimeRange succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingSlotsCountKey, len(slots)),
	)
	return slots, nil
}

// FindSlotsByScheduleWithQuery fetches slots by schedule with supplied search params.
// Only whitelisted keys are appended: such as start, end, status (exactly as FHIR expects, e.g., start=lt2025-...)
// Caller builds the comparator in the value (lt,gt,eq).
func (c *slotFhirClient) FindSlotsByScheduleWithQuery(ctx context.Context, scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.FindSlotsByScheduleWithQuery called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, scheduleID),
	)

	base := fmt.Sprintf("%s?schedule=Schedule/%s", c.BaseUrl, url.QueryEscape(scheduleID))
	queryURL := base + params.ToQueryString()

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, queryURL, nil)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotsByScheduleWithQuery error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotsByScheduleWithQuery error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, rerr := io.ReadAll(resp.Body)
		if rerr != nil {
			c.Log.Error("slotFhirClient.FindSlotsByScheduleWithQuery error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(rerr),
			)
			return nil, exceptions.ErrGetFHIRResource(rerr, constvars.ResourceSlot)
		}
		var outcome fhir_dto.OperationOutcome
		if uerr := json.Unmarshal(bodyBytes, &outcome); uerr != nil {
			c.Log.Error("slotFhirClient.FindSlotsByScheduleWithQuery error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(uerr),
			)
			return nil, exceptions.ErrGetFHIRResource(uerr, constvars.ResourceSlot)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("slotFhirClient.FindSlotsByScheduleWithQuery FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	if derr := json.NewDecoder(resp.Body).Decode(&result); derr != nil {
		c.Log.Error("slotFhirClient.FindSlotsByScheduleWithQuery error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(derr),
		)
		return nil, exceptions.ErrDecodeResponse(derr, constvars.ResourceSlot)
	}

	out := make([]fhir_dto.Slot, len(result.Entry))
	for i, e := range result.Entry {
		out[i] = e.Resource
	}
	c.Log.Info("slotFhirClient.FindSlotsByScheduleWithQuery succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingSlotsCountKey, len(out)),
	)
	return out, nil
}

// PostTransactionBundle posts a transaction bundle to the FHIR base endpoint and returns the response bundle.
func (c *slotFhirClient) PostTransactionBundle(ctx context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	// BaseUrl points to .../Slot; trim to base
	base := strings.TrimSuffix(c.BaseUrl, constvars.ResourceSlot)

	body, err := json.Marshal(bundle)
	if err != nil {
		c.Log.Error("slotFhirClient.PostTransactionBundle error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, base, bytes.NewBuffer(body))
	if err != nil {
		c.Log.Error("slotFhirClient.PostTransactionBundle error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("slotFhirClient.PostTransactionBundle error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK && resp.StatusCode != constvars.StatusCreated {
		bodyBytes, rerr := io.ReadAll(resp.Body)
		if rerr != nil {
			c.Log.Error("slotFhirClient.PostTransactionBundle error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(rerr),
			)
			return nil, exceptions.ErrCreateFHIRResource(rerr, constvars.ResourceSlot)
		}
		var outcome fhir_dto.OperationOutcome
		if uerr := json.Unmarshal(bodyBytes, &outcome); uerr != nil {
			c.Log.Error("slotFhirClient.PostTransactionBundle error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(uerr),
			)
			return nil, exceptions.ErrCreateFHIRResource(uerr, constvars.ResourceSlot)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("slotFhirClient.PostTransactionBundle FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	var result fhir_dto.FHIRBundle
	if derr := json.NewDecoder(resp.Body).Decode(&result); derr != nil {
		c.Log.Error("slotFhirClient.PostTransactionBundle error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(derr),
		)
		return nil, exceptions.ErrDecodeResponse(derr, constvars.ResourceSlot)
	}

	c.Log.Info("slotFhirClient.PostTransactionBundle succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingSlotsCountKey, len(result.Entry)),
	)
	return &result, nil
}
