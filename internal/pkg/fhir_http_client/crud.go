package fhir_http_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"

	"go.uber.org/zap"
)

// CreateResource marshals resource, POSTs to baseUrl, unmarshals the response, and returns the created resource.
// It logs entry/error/success using the provided logger and resourceName.
func CreateResource[T any](ctx context.Context, log *zap.Logger, client *FHIRHTTPClient,
	baseUrl string, resource *T, resourceName, idLogKey string) (*T, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	log.Info("crud.CreateResource called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("resource", resourceName),
	)

	reqJSON, err := json.Marshal(resource)
	if err != nil {
		log.Error("crud.CreateResource error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := client.Do(ctx, constvars.MethodPost, baseUrl, bytes.NewBuffer(reqJSON))
	if err != nil {
		log.Error("crud.CreateResource FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, resourceName)
	}

	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Error("crud.CreateResource error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, resourceName)
	}

	// Extract the ID from the response for logging (all FHIR DTOs have an "id" field)
	var idExtract struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &idExtract)
	if idExtract.ID != "" {
		log.Info("crud.CreateResource succeeded",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(idLogKey, idExtract.ID),
		)
	} else {
		log.Info("crud.CreateResource succeeded",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	}
	return &result, nil
}

// GetResource GETs {baseUrl}/{id}, unmarshals the response, and returns the resource.
func GetResource[T any](ctx context.Context, log *zap.Logger, client *FHIRHTTPClient,
	baseUrl, id, resourceName, idLogKey string) (*T, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	log.Info("crud.GetResource called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("resource", resourceName),
		zap.String(idLogKey, id),
	)

	respBody, err := client.Do(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", baseUrl, id), nil)
	if err != nil {
		log.Error("crud.GetResource FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, resourceName)
	}

	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Error("crud.GetResource error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, resourceName)
	}

	log.Info("crud.GetResource succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(idLogKey, id),
	)
	return &result, nil
}

// WriteResourceInput bundles all parameters for WriteResource into a single struct
// to avoid exceeding the function parameter limit.
type WriteResourceInput[T any] struct {
	Ctx          context.Context
	Log          *zap.Logger
	Client       *FHIRHTTPClient
	Method       string
	BaseUrl      string
	ID           string
	Resource     *T
	ResourceName string
	IDLogKey     string
}

// WriteResource marshals resource, sends it to {baseUrl}/{id} via the specified HTTP method
// (typically constvars.MethodPut or constvars.MethodPatch), unmarshals the response, and returns the result.
func WriteResource[T any](input WriteResourceInput[T]) (*T, error) {
	requestID, _ := input.Ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	input.Log.Info("crud.WriteResource called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("resource", input.ResourceName),
		zap.String(input.IDLogKey, input.ID),
	)

	reqJSON, err := json.Marshal(input.Resource)
	if err != nil {
		input.Log.Error("crud.WriteResource error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := input.Client.Do(input.Ctx, input.Method, fmt.Sprintf("%s/%s", input.BaseUrl, input.ID), bytes.NewBuffer(reqJSON))
	if err != nil {
		input.Log.Error("crud.WriteResource FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, input.ResourceName)
	}

	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		input.Log.Error("crud.WriteResource error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, input.ResourceName)
	}

	input.Log.Info("crud.WriteResource succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(input.IDLogKey, input.ID),
	)
	return &result, nil
}

// DeleteResource DELETEs {baseUrl}/{id}. Returns nil on success.
func DeleteResource(ctx context.Context, log *zap.Logger, client *FHIRHTTPClient,
	baseUrl, id, resourceName string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	log.Info("crud.DeleteResource called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("resource", resourceName),
	)

	_, err := client.Do(ctx, constvars.MethodDelete, fmt.Sprintf("%s/%s", baseUrl, id), nil)
	if err != nil {
		log.Error("crud.DeleteResource FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrGetFHIRResource(err, resourceName)
	}

	log.Info("crud.DeleteResource succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

// SearchResources GETs urlStr, decodes the FHIR bundle entries into []T, and returns them.
func SearchResources[T any](ctx context.Context, log *zap.Logger, client *FHIRHTTPClient,
	urlStr, resourceName string) ([]T, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	log.Info("crud.SearchResources called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("resource", resourceName),
	)

	respBody, err := client.Do(ctx, constvars.MethodGet, urlStr, nil)
	if err != nil {
		log.Error("crud.SearchResources FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, resourceName)
	}

	var bundle fhir_dto.FHIRBundle
	if err := json.Unmarshal(respBody, &bundle); err != nil {
		log.Error("crud.SearchResources error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, resourceName)
	}

	out := make([]T, 0, len(bundle.Entry))
	for _, entry := range bundle.Entry {
		var resource T
		if err := json.Unmarshal(entry.Resource, &resource); err != nil {
			log.Error("crud.SearchResources error unmarshaling entry",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrDecodeResponse(err, resourceName)
		}
		out = append(out, resource)
	}

	log.Info("crud.SearchResources succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int("count", len(out)),
	)
	return out, nil
}
