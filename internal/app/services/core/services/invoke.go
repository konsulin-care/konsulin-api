package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/google/uuid"
	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// WebhookIdentifierSystem is the FHIR identifier system under which the
// invocation's webhook id is recorded on the async ServiceRequest receipt.
const WebhookIdentifierSystem = "urn:konsulin:webhook-id"

// Invocation modes accepted by Invoke.
const (
	// InvokeModeAsync queues the invocation at the gateway and records a
	// ServiceRequest receipt, returning its id.
	InvokeModeAsync = "async"
	// InvokeModeSync forwards the invocation inline and returns the opened
	// reply.
	InvokeModeSync = "sync"
)

// ContextKey is the private key type for invocation context values set by
// the backend's own middleware, never by the client body.
type ContextKey string

// Context values Invoke reads to build the backend-owned invocation context.
const (
	// ContextKeyUserRef carries the peppered end-user hash (user_ref).
	ContextKeyUserRef ContextKey = "invoke_user_ref"
	// ContextKeySubjectRef carries the FHIR subject reference the
	// invocation runs on behalf of.
	ContextKeySubjectRef ContextKey = "invoke_subject_ref"
)

// InvocationServiceRequest is the async invocation receipt to persist as a
// FHIR ServiceRequest: the webhook id identifier plus the requester.
type InvocationServiceRequest struct {
	// Identifier records the webhook id under urn:konsulin:webhook-id.
	Identifier []fhir_dto.Identifier
	// Requester is the caller's FHIR reference.
	Requester string
	// Subject is the invocation's subject reference ("" when unknown).
	Subject string
}

// ServiceRequestRecorder persists one async invocation receipt and returns
// the created ServiceRequest id.
type ServiceRequestRecorder interface {
	// RecordInvocation stores receipt and returns the new ServiceRequest
	// id, or an error explaining the failed write.
	RecordInvocation(ctx context.Context, receipt *InvocationServiceRequest) (string, error)
}

// InvokeOutput carries the mode-dependent result of one invocation: the
// ServiceRequest id for async modes, the opened reply for sync modes.
type InvokeOutput struct {
	// ServiceRequestID is the async receipt id ("" for sync invocations).
	ServiceRequestID string
	// Reply is the opened sync reply (nil for async invocations).
	Reply []byte
}

// PaymentRequiredError reports a gateway 402 for a paid listing: it carries
// the listing's id, price and price_scope so callers can render the payment
// prompt. It unwraps to a 402 *exceptions.CustomError for the HTTP layer.
type PaymentRequiredError struct {
	// ListingID is the active listing the gateway refused to invoke.
	ListingID string
	// Price is the listing price in IDR minor units (0 marks a free listing).
	Price int64
	// PriceScope is "party" or "user".
	PriceScope string
}

// Error renders the payment requirement as a single line.
// Returns: the human-readable message. Never panics.
func (e *PaymentRequiredError) Error() string {
	return fmt.Sprintf("payment required for listing %s (price %d, price_scope %s)",
		e.ListingID, e.Price, e.PriceScope)
}

// Unwrap exposes the renderable 402 custom error for the HTTP error path.
// Returns: a *exceptions.CustomError with status 402 carrying the listing
// details. Never panics.
func (e *PaymentRequiredError) Unwrap() error {
	return exceptions.BuildNewCustomError(nil, constvars.StatusPaymentRequired,
		fmt.Sprintf("payment required for listing %s (price %d, %s scope)",
			e.ListingID, e.Price, e.PriceScope),
		fmt.Sprintf("GATEWAY_PAYMENT_REQUIRED listing_id=%s price=%d price_scope=%s",
			e.ListingID, e.Price, e.PriceScope))
}

// Invoker executes marketplace service invocations through the gateway
// webhook endpoints: it seals a backend-built data envelope, dispatches it
// async (enqueue plus ServiceRequest receipt) or sync (opened reply), and
// maps a gateway 402 to a payment-required error carrying the listing.
type Invoker struct {
	hooks        contracts.WebhookClient
	listings     contracts.ListingDirectory
	recorder     ServiceRequestRecorder
	newWebhookID func() string
}

// NewInvoker builds an Invoker over the gateway webhook client, the listing
// directory used to enrich 402 answers and the async receipt recorder.
//
// Parameters:
//   - hooks routes the async enqueue and the sync forward.
//   - listings resolves the paid listing behind a gateway 402 (may be nil).
//   - recorder persists the async ServiceRequest receipt (may be nil only
//     when async invocations are never used).
//
// Returns: an Invoker ready to serve Invoke. Never panics.
func NewInvoker(hooks contracts.WebhookClient, listings contracts.ListingDirectory,
	recorder ServiceRequestRecorder,
) *Invoker {
	return &Invoker{hooks: hooks, listings: listings, recorder: recorder}
}

// Invoke runs one service invocation: it builds the sealed data envelope
// {input, context} from the client body and the backend-owned context
// (client-supplied context members are discarded), then dispatches by mode.
// Async mode enqueues at the gateway, records a ServiceRequest identified by
// the webhook id with the caller as requester, and returns its id; sync mode
// returns the opened reply. A gateway 402 surfaces as a 402 carrying the
// listing's id, price and price_scope.
//
// Parameters:
//   - ctx carries the backend context (request id, roles, user_ref,
//     subject_ref) and bounds the gateway calls.
//   - caller is the requester's FHIR reference on the async receipt.
//   - provider is the providing party id or alias.
//   - name is the listing name (gateway service).
//   - body is the client request body carried verbatim as input.
//   - mode is InvokeModeAsync or InvokeModeSync (case-insensitive).
//
// Returns: the invocation output, or an error — a 402
// *PaymentRequiredError for a paid listing the gateway refuses, a 400 for
// an unknown mode, the gateway error otherwise. Never panics.
func (inv *Invoker) Invoke(ctx context.Context, caller, provider, name string,
	body []byte, mode string,
) (*InvokeOutput, error) {
	if inv == nil || inv.hooks == nil {
		return nil, contracts.ErrGatewayNotConfigured
	}
	data, err := buildInvocationData(ctx, body)
	if err != nil {
		return nil, err
	}
	userRef := ctxString(ctx, ContextKeyUserRef)

	switch strings.ToLower(strings.TrimSpace(mode)) {
	case InvokeModeAsync:
		if err := inv.hooks.Enqueue(ctx, provider, name, userRef, data); err != nil {
			return nil, inv.gatewayError(ctx, provider, name, err)
		}
		id, err := inv.record(ctx, caller, ctxString(ctx, ContextKeySubjectRef))
		if err != nil {
			return nil, err
		}
		return &InvokeOutput{ServiceRequestID: id}, nil
	case InvokeModeSync:
		reply, err := inv.hooks.Sync(ctx, provider, name, userRef, data)
		if err != nil {
			return nil, inv.gatewayError(ctx, provider, name, err)
		}
		return &InvokeOutput{Reply: reply}, nil
	default:
		return nil, exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest,
			"unsupported invocation mode", "INVOCATION_MODE_UNSUPPORTED")
	}
}

// invocationData is the sealed payload handed to the gateway webhook client.
type invocationData struct {
	Input   json.RawMessage   `json:"input"`
	Context invocationContext `json:"context"`
}

// invocationContext is the backend-owned invocation context: every member is
// derived from the request context, never from the client body.
type invocationContext struct {
	UserRef    string `json:"user_ref"`
	Role       string `json:"role"`
	SubjectRef string `json:"subject_ref"`
	RequestID  string `json:"request_id"`
}

// buildInvocationData seals the client body as input next to the
// backend-built context. A non-JSON body is carried as a JSON string.
//
// Parameters:
//   - ctx supplies the backend context members.
//   - body is the client request body.
//
// Returns: the marshalled envelope, or an error when encoding fails.
// Never panics.
func buildInvocationData(ctx context.Context, body []byte) ([]byte, error) {
	input := json.RawMessage(body)
	if !json.Valid(input) {
		quoted, err := json.Marshal(string(body))
		if err != nil {
			return nil, exceptions.ErrCannotMarshalJSON(err)
		}
		input = quoted
	}
	data := invocationData{
		Input: input,
		Context: invocationContext{
			UserRef:    ctxString(ctx, ContextKeyUserRef),
			Role:       strings.Join(ctxStrings(ctx, constvars.CONTEXT_FHIR_ROLE), ","),
			SubjectRef: ctxString(ctx, ContextKeySubjectRef),
			RequestID:  ctxString(ctx, constvars.CONTEXT_REQUEST_ID_KEY),
		},
	}
	out, err := json.Marshal(data)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}
	return out, nil
}

// ctxString reads a string context value; a missing or non-string value
// yields "".
func ctxString(ctx context.Context, key any) string {
	s, _ := ctx.Value(key).(string)
	return s
}

// ctxStrings reads a []string context value; a missing or mismatched value
// yields nil.
func ctxStrings(ctx context.Context, key any) []string {
	s, _ := ctx.Value(key).([]string)
	return s
}

// record mints the webhook id and persists the async receipt, returning the
// created ServiceRequest id.
//
// Parameters:
//   - ctx bounds the write, caller is the requester reference, subjectRef
//     the optional subject reference.
//
// Returns: the ServiceRequest id, or the recorder error. Never panics.
func (inv *Invoker) record(ctx context.Context, caller, subjectRef string) (string, error) {
	if inv.recorder == nil {
		return "", errors.New("invoke: no service request recorder configured")
	}
	mint := inv.newWebhookID
	if mint == nil {
		mint = uuid.NewString
	}
	receipt := &InvocationServiceRequest{
		Identifier: []fhir_dto.Identifier{
			{System: WebhookIdentifierSystem, Value: mint()},
		},
		Requester: caller,
		Subject:   subjectRef,
	}
	return inv.recorder.RecordInvocation(ctx, receipt)
}

// gatewayError maps a gateway failure: a 402 answer is enriched with the
// listing's id, price and price_scope through the listing directory and
// returned as *PaymentRequiredError; every other error passes through raw.
//
// Parameters:
//   - ctx bounds the listing lookup, provider and name select the listing,
//     err is the gateway error.
//
// Returns: the mapped or original error. Never panics.
func (inv *Invoker) gatewayError(ctx context.Context, provider, name string, err error) error {
	var werr *wire.Error
	if !errors.As(err, &werr) || werr.Status != constvars.StatusPaymentRequired {
		return err
	}
	out := &PaymentRequiredError{}
	if inv.listings != nil {
		if listing, lerr := inv.listings.FindListing(ctx, provider, name); lerr == nil && listing != nil {
			out.ListingID = listing.ListingID
			out.Price = listing.Price
			out.PriceScope = listing.PriceScope
		}
	}
	return out
}

// fhirRecorder persists invocation receipts through the FHIR ServiceRequest
// client: it creates the resource, then attaches the webhook id identifier
// with an update, because the create input carries no identifier member.
type fhirRecorder struct {
	client contracts.ServiceRequestFhirClient
}

// NewFHIRServiceRequestRecorder adapts a contracts.ServiceRequestFhirClient
// to the ServiceRequestRecorder seam: it creates the receipt ServiceRequest
// (requester from the caller, subject when known) and attaches the webhook
// id identifier through an update, returning the created id.
//
// Parameters:
//   - client is the FHIR ServiceRequest client.
//
// Returns: a recorder ready to bind. Never panics.
func NewFHIRServiceRequestRecorder(client contracts.ServiceRequestFhirClient) ServiceRequestRecorder {
	return &fhirRecorder{client: client}
}

// RecordInvocation creates the receipt ServiceRequest and stamps the
// webhook id identifier on it.
//
// Parameters:
//   - ctx bounds the FHIR calls, receipt carries identifier, requester and
//     optional subject.
//
// Returns: the created ServiceRequest id, or the FHIR error. Never panics.
func (r *fhirRecorder) RecordInvocation(ctx context.Context, receipt *InvocationServiceRequest) (string, error) {
	create := &fhir_dto.CreateServiceRequestInput{
		ResourceType: constvars.ResourceServiceRequest,
		Status:       "active",
		Intent:       "order",
		AuthoredOn:   time.Now().UTC(),
	}
	if receipt.Requester != "" {
		create.Requester = &fhir_dto.Reference{Reference: receipt.Requester}
	}
	if receipt.Subject != "" {
		create.Subject = fhir_dto.Reference{Reference: receipt.Subject}
	}
	created, err := r.client.CreateServiceRequest(ctx, create)
	if err != nil {
		return "", err
	}
	if created == nil || strings.TrimSpace(created.ID) == "" {
		return "", exceptions.ErrCreateFHIRResource(nil, constvars.ResourceServiceRequest)
	}
	update := &fhir_dto.UpdateServiceRequestInput{
		ResourceType:    constvars.ResourceServiceRequest,
		ID:              created.ID,
		Meta:            created.Meta,
		Status:          created.Status,
		Intent:          created.Intent,
		Subject:         created.Subject,
		Requester:       create.Requester,
		AuthoredOn:      created.AuthoredOn,
		InstantiatesUri: created.InstantiatesUri,
		Note:            created.Note,
		Identifier:      receipt.Identifier,
	}
	if _, err := r.client.Update(ctx, created.ID, update); err != nil {
		return "", err
	}
	return created.ID, nil
}
