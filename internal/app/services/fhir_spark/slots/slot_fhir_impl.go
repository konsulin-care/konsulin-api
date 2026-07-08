package slots

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
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
	*base.ResourceClient
}

func NewSlotFhirClient(baseUrl string, logger *zap.Logger) contracts.SlotFhirClient {
	onceSlotFhirClient.Do(func() {
		slotFhirClientInstance = &slotFhirClient{
			ResourceClient: base.New(baseUrl, constvars.ResourceSlot, logger),
		}
	})
	return slotFhirClientInstance
}

func (c *slotFhirClient) FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("slotFhirClient.FindSlotByScheduleID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, scheduleID),
	)

	respBody, err := c.Client.Do(ctx, constvars.MethodGet,
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
	return fhir_http_client.GetResource[fhir_dto.Slot](ctx, c.Log, c.Client, c.BaseUrl, slotID,
		constvars.ResourceSlot, constvars.LoggingSlotsIDKey)
}

func (c *slotFhirClient) FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]fhir_dto.Slot, error) {
	url := fmt.Sprintf("%s?schedule=Schedule/%s&status=%s", c.BaseUrl, scheduleID, status)
	return fhir_http_client.SearchResources[fhir_dto.Slot](ctx, c.Log, c.Client, url,
		constvars.ResourceSlot)
}

func (c *slotFhirClient) CreateSlot(ctx context.Context, request *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return fhir_http_client.CreateResource(ctx, c.Log, c.Client, c.BaseUrl, request,
		constvars.ResourceSlot, constvars.LoggingSlotsIDKey)
}

func (c *slotFhirClient) UpdateSlot(ctx context.Context, id string, slot *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return fhir_http_client.WriteResource(ctx, c.Log, c.Client, constvars.MethodPut, c.BaseUrl, id, slot,
		constvars.ResourceSlot, constvars.LoggingSlotsIDKey)
}

func (c *slotFhirClient) FindSlotByScheduleAndTimeRange(ctx context.Context, scheduleID string, startTime, endTime time.Time) ([]fhir_dto.Slot, error) {
	queryURL := fmt.Sprintf(
		"%s?schedule=Schedule/%s&start=eq%s&end=eq%s",
		c.BaseUrl,
		scheduleID,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
	)
	return fhir_http_client.SearchResources[fhir_dto.Slot](ctx, c.Log, c.Client, queryURL,
		constvars.ResourceSlot)
}

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
		respBody, err := c.Client.Do(ctx, constvars.MethodGet, nextURL, nil)
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

func (c *slotFhirClient) PostTransactionBundle(ctx context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	base := strings.TrimSuffix(c.BaseUrl, constvars.ResourceSlot)

	body, err := json.Marshal(bundle)
	if err != nil {
		c.Log.Error("slotFhirClient.PostTransactionBundle error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.Client.Do(ctx, constvars.MethodPost, base, bytes.NewBuffer(body))
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
