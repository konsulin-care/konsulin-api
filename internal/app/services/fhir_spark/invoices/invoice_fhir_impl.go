package invoices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"sync"

	"go.uber.org/zap"
)

var (
	invoiceFhirClientInstance contracts.InvoiceFhirClient
	onceInvoiceFhirClient     sync.Once
)

type invoiceFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewInvoiceFhirClient(baseUrl string, logger *zap.Logger) contracts.InvoiceFhirClient {
	onceInvoiceFhirClient.Do(func() {
		client := &invoiceFhirClient{
			BaseUrl: baseUrl + constvars.ResourceInvoice,
			Log:     logger,
		}
		invoiceFhirClientInstance = client
	})
	return invoiceFhirClientInstance
}

func (c *invoiceFhirClient) Search(ctx context.Context, params contracts.InvoiceSearchParams) ([]fhir_dto.Invoice, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("invoiceFhirClient.Search called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any("params", params),
	)

	queryString := params.ToQueryParam().Encode()
	url := c.BaseUrl + "?" + queryString

	c.Log.Info("invoiceFhirClient.Search built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("url", url),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("invoiceFhirClient.Search error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("invoiceFhirClient.Search error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("invoiceFhirClient.Search error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceInvoice)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("invoiceFhirClient.Search error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceInvoice)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("invoiceFhirClient.Search FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceInvoice)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string           `json:"fullUrl"`
			Resource fhir_dto.Invoice `json:"resource"`
		} `json:"entry"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("invoiceFhirClient.Search error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceInvoice)
	}

	invoices := make([]fhir_dto.Invoice, len(result.Entry))
	for i, entry := range result.Entry {
		invoices[i] = entry.Resource
	}

	c.Log.Info("invoiceFhirClient.Search succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int("count", len(invoices)),
	)

	return invoices, nil
}
