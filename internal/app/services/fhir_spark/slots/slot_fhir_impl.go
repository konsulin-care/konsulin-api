package slots

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"
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
	client  *fhir_http_client.FHIRHTTPClient
}

func NewSlotFhirClient(baseUrl string, logger *zap.Logger) contracts.SlotFhirClient {
	onceSlotFhirClient.Do(func() {
		client := &slotFhirClient{
			BaseUrl: baseUrl + constvars.ResourceSlot,
			Log:     logger,
			client:  fhir_http_client.New(logger),
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

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/schedule=Schedule/%s", c.BaseUrl, scheduleID), nil)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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

func (c *slotFhirClient) FindSlotByID(ctx context.Context, slotID string) (*fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.FindSlotByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", slotID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/%s", c.BaseUrl, slotID), nil)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
	}

	var slot fhir_dto.Slot
	if err := json.Unmarshal(respBody, &slot); err != nil {
		c.Log.Error("slotFhirClient.FindSlotByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	c.Log.Info("slotFhirClient.FindSlotByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", slot.ID),
	)
	return &slot, nil
}

func (c *slotFhirClient) FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.FindSlotByScheduleIDAndStatus called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, scheduleID),
		zap.String(constvars.LoggingScheduleStatusKey, status),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?schedule=Schedule/%s&status=%s", c.BaseUrl, scheduleID, status), nil)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleIDAndStatus FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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

	respBody, err := c.client.Do(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("slotFhirClient.CreateSlot FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceSlot)
	}

	var slotFhir fhir_dto.Slot
	if err := json.Unmarshal(respBody, &slotFhir); err != nil {
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
	return &slotFhir, nil
}

func (c *slotFhirClient) UpdateSlot(ctx context.Context, id string, slot *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.UpdateSlot called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("id", id),
	)

	requestJSON, err := json.Marshal(slot)
	if err != nil {
		c.Log.Error("slotFhirClient.UpdateSlot error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.client.Do(ctx, constvars.MethodPut,
		fmt.Sprintf("%s/%s", c.BaseUrl, id),
		bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("slotFhirClient.UpdateSlot FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceSlot)
	}

	var out fhir_dto.Slot
	if err := json.Unmarshal(respBody, &out); err != nil {
		c.Log.Error("slotFhirClient.UpdateSlot error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	c.Log.Info("slotFhirClient.UpdateSlot succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slot_id", out.ID),
	)
	return &out, nil
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

	respBody, err := c.client.Do(ctx, constvars.MethodGet, queryURL, nil)
	if err != nil {
		c.Log.Error("slotFhirClient.FindSlotByScheduleAndTimeRange FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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

// decodeSlotBundle decodes a single FHIR Slot searchset bundle page from data and returns
// the slot entries and the "next" link URL if present. Uses fhir_dto.FHIRBundle (with Link).
func decodeSlotBundle(data []byte) ([]fhir_dto.Slot, string, error) {
	var result fhir_dto.FHIRBundle
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, "", err
	}
	out := make([]fhir_dto.Slot, 0, len(result.Entry))
	for _, e := range result.Entry {
		var slot fhir_dto.Slot
		if err := json.Unmarshal(e.Resource, &slot); err != nil {
			return nil, "", err
		}
		out = append(out, slot)
	}
	var nextURL string
	for _, l := range result.Link {
		if l.Relation == "next" && l.Url != "" {
			nextURL = l.Url
			break
		}
	}
	return out, nextURL, nil
}

// FindSlotsByScheduleWithQuery fetches slots by schedule with supplied search params.
// It follows FHIR bundle "next" links until all pages are retrieved, then returns the aggregated list.
// Caller builds the comparator in the value (lt,gt,eq).
func (c *slotFhirClient) FindSlotsByScheduleWithQuery(ctx context.Context, scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.FindSlotsByScheduleWithQuery called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, scheduleID),
	)

	base := fmt.Sprintf("%s?schedule=Schedule/%s", c.BaseUrl, url.QueryEscape(scheduleID))
	queryURL := base + params.ToQueryString()

	var out []fhir_dto.Slot
	nextURL := queryURL

	for {
		respBody, err := c.client.Do(ctx, constvars.MethodGet, nextURL, nil)
		if err != nil {
			c.Log.Error("slotFhirClient.FindSlotsByScheduleWithQuery FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}

		pageSlots, next, decErr := decodeSlotBundle(respBody)
		if decErr != nil {
			c.Log.Error("slotFhirClient.FindSlotsByScheduleWithQuery error decoding response",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(decErr),
			)
			return nil, exceptions.ErrDecodeResponse(decErr, constvars.ResourceSlot)
		}

		out = append(out, pageSlots...)
		if next == "" {
			break
		}
		nextURL = next
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

	respBody, err := c.client.Do(ctx, constvars.MethodPost, base, bytes.NewBuffer(body))
	if err != nil {
		c.Log.Error("slotFhirClient.PostTransactionBundle FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceSlot)
	}

	var result fhir_dto.FHIRBundle
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("slotFhirClient.PostTransactionBundle error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	c.Log.Info("slotFhirClient.PostTransactionBundle succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingSlotsCountKey, len(result.Entry)),
	)
	return &result, nil
}
