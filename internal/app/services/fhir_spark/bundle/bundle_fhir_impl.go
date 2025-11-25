package bundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"

	"go.uber.org/zap"
)

type BundleFhirClient interface {
	// PostTransactionBundle posts a transaction bundle to the FHIR base endpoint and returns the response bundle, plain and simple.
	PostTransactionBundle(ctx context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error)
}

type BundleFhirClientImpl struct {
	baseFhirURL string
	log         *zap.Logger
}

// NewBundleFhirClient returns a concrete client. Callers can still depend on the
// BundleFhirClient interface for abstraction.
func NewBundleFhirClient(baseFhirURL string, logger *zap.Logger) *BundleFhirClientImpl {
	return &BundleFhirClientImpl{baseFhirURL: baseFhirURL, log: logger}
}

func (c *BundleFhirClientImpl) PostTransactionBundle(ctx context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	requestJSON, err := json.Marshal(bundle)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.baseFhirURL, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK && resp.StatusCode != constvars.StatusCreated {
		bodyBytes, rerr := io.ReadAll(resp.Body)
		if rerr != nil {
			c.log.Error("bundleFhirClient.PostTransactionBundle error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(rerr),
			)
			return nil, exceptions.ErrCreateFHIRResource(rerr, "Bundle")
		}
		var outcome fhir_dto.OperationOutcome
		if uerr := json.Unmarshal(bodyBytes, &outcome); uerr != nil {
			c.log.Error("bundleFhirClient.PostTransactionBundle error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(uerr),
			)
			return nil, exceptions.ErrCreateFHIRResource(uerr, "Bundle")
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.log.Error("bundleFhirClient.PostTransactionBundle FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, "Bundle")
		}
	}

	var result fhir_dto.FHIRBundle
	if derr := json.NewDecoder(resp.Body).Decode(&result); derr != nil {
		return nil, exceptions.ErrDecodeResponse(derr, "Bundle")
	}
	return &result, nil
}
