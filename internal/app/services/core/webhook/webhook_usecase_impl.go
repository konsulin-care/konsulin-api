package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/app/services/shared/webhookqueue"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"strings"
	"time"

	"github.com/google/uuid"

	"slices"

	"go.uber.org/zap"
)

// Usecase exposes operations for webhook service integration.
type Usecase interface {
	// Enqueue stores the incoming request in the durable queue after validation and rate limiting.
	Enqueue(ctx context.Context, in *EnqueueInput) (*EnqueueOutput, error)
	// HandleAsyncServiceResult processes callback results for async service requests.
	HandleAsyncServiceResult(ctx context.Context, in *HandleAsyncServiceResultInput) error
	// GetAsyncServiceResult retrieves the result of an async service request.
	GetAsyncServiceResult(ctx context.Context, id string) (*GetAsyncServiceResultOutput, error)
}

type usecase struct {
	log                *zap.Logger
	cfg                *config.InternalConfig
	queue              *webhookqueue.Service
	jwtManager         *jwtmanager.JWTManager
	patientFhir        contracts.PatientFhirClient
	practitionerFhir   contracts.PractitionerFhirClient
	serviceRequestFhir contracts.ServiceRequestFhirClient
}

// NewUsecase creates a new webhook usecase instance.
func NewUsecase(log *zap.Logger, cfg *config.InternalConfig, queue *webhookqueue.Service, jwtMgr *jwtmanager.JWTManager, patient contracts.PatientFhirClient, practitioner contracts.PractitionerFhirClient, sr contracts.ServiceRequestFhirClient) Usecase {
	return &usecase{
		log:                log,
		cfg:                cfg,
		queue:              queue,
		jwtManager:         jwtMgr,
		patientFhir:        patient,
		practitionerFhir:   practitioner,
		serviceRequestFhir: sr,
	}
}

// EnqueueInput captures request details for enqueueing.
type EnqueueInput struct {
	ServiceName string
	Method      string
	RawJSON     json.RawMessage
}

// EnqueueOutput for webhook enqueue requests
type EnqueueOutput struct {
	AsyncServiceResultID string `json:"asyncServiceResultId,omitempty"`
}

// HandleAsyncServiceResultInput captures input for async service result callback.
type HandleAsyncServiceResultInput struct {
	ServiceRequestID string
	Result           string
	Timestamp        time.Time
}

// validate checks that all required fields are present.
func (in *HandleAsyncServiceResultInput) validate() error {
	if strings.TrimSpace(in.ServiceRequestID) == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "serviceRequestId is required", "VALIDATION_ERROR")
	}
	if strings.TrimSpace(in.Result) == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "result is required", "VALIDATION_ERROR")
	}
	if in.Timestamp.IsZero() {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "timestamp is required", "VALIDATION_ERROR")
	}
	return nil
}

// GetAsyncServiceResultOutput represents the output for async service result retrieval.
type GetAsyncServiceResultOutput struct {
	ResourceType    string             `json:"resourceType"`
	Status          string             `json:"status"`
	Intent          string             `json:"intent"`
	InstantiatesUri []string           `json:"instantiatesUri"`
	Subject         fhir_dto.Reference `json:"subject"`
	Requester       fhir_dto.Reference `json:"requester"`
	AuthoredOn      time.Time          `json:"authoredOn"`
	Note            string             `json:"note"`
}

// JWTForwardedFromPaymentServiceHeader is a special header key that will be checked to
// ensure the request comes from trusted payment service.
const JWTForwardedFromPaymentServiceHeader = "X-Forwarded-From-Payment-Service"

// PAYMENT_SERVICE_SUB is the expected JWT subject for forwarded requests from payment service
const PAYMENT_SERVICE_SUB = "payment-service"

// Enqueue validates, rate-limits, and enqueues the message.
func (u *usecase) Enqueue(ctx context.Context, in *EnqueueInput) (*EnqueueOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	u.log.Info("webhook.usecase.Enqueue called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_name", in.ServiceName),
		zap.String("method", in.Method),
	)

	// Auth gate: support forwarded JWT header from payment service
	forwarded := ""
	if v := ctx.Value(JWTForwardedFromPaymentServiceHeader); v != nil {
		if s, ok := v.(string); ok {
			forwarded = s
		}
	}
	if err := u.evaluateWebhookAuth(ctx, &evaluateAuthInput{ServiceName: in.ServiceName, ForwardedJWT: forwarded}); err != nil {
		return nil, err
	}

	// Only POST allowed per requirement
	if in.Method != constvars.MethodPost {
		return nil, exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusMethodNotAllowed, "Only POST is allowed", "WEBHOOK_METHOD_NOT_ALLOWED"))
	}

	// Async ServiceRequest flow (short-circuit)
	if u.isAsyncService(in.ServiceName) {
		return u.handleAsyncService(ctx, in)
	}

	msg := webhookqueue.WebhookQueueMessage{
		ID:          uuid.NewString(),
		Method:      in.Method,
		ServiceName: in.ServiceName,
		Body:        in.RawJSON,
		FailedCount: 0,
	}
	_, err := u.queue.Enqueue(ctx, &webhookqueue.EnqueueToWebhookServiceQueueInput{Message: msg})
	if err != nil {
		return nil, err
	}

	return &EnqueueOutput{}, nil
}

func (u *usecase) isAsyncService(svc string) bool {
	if strings.TrimSpace(svc) == "" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(svc))
	return slices.Contains(u.cfg.Webhook.AsyncServiceNames, target)
}

// determineSubjectFromRequester returns a FHIR reference string:
// - "Group/guest" for Guest
// - "Patient/{id}" for Patient
// - "Practitioner/{id}" for Practitioner
func (u *usecase) determineSubjectFromRequester(ctx context.Context) (string, error) {
	roles := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	uid := ctx.Value(constvars.CONTEXT_UID).(string)

	// Guest
	for _, r := range roles {
		if strings.EqualFold(r, constvars.KonsulinRoleGuest) {
			return string(constvars.ServiceRequestSubjectGuest), nil
		}
	}

	// Prefer Practitioner over Patient if both present
	if slices.Contains(roles, constvars.KonsulinRolePractitioner) {
		if strings.TrimSpace(uid) == "" {
			return "", exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "missing uid in context", "MISSING_UID"))
		}
		pracs, err := u.practitionerFhir.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, uid)
		if err != nil {
			return "", err
		}
		if len(pracs) < 1 {
			return "", exceptions.ErrGetFHIRResource(nil, constvars.ResourcePractitioner)
		}
		return fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, pracs[0].ID), nil
	}

	if slices.Contains(roles, constvars.KonsulinRolePatient) {
		if strings.TrimSpace(uid) == "" {
			return "", exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "missing uid in context", "MISSING_UID"))
		}
		ident := constvars.FhirSupertokenSystemIdentifier + "|" + uid
		pats, err := u.patientFhir.FindPatientByIdentifier(ctx, ident)
		if err != nil {
			return "", err
		}
		if len(pats) < 1 {
			return "", exceptions.ErrGetFHIRResource(nil, constvars.ResourcePatient)
		}
		return fmt.Sprintf("%s/%s", constvars.ResourcePatient, pats[0].ID), nil
	}

	// Fallback to guest
	return string(constvars.ServiceRequestSubjectGuest), nil
}

func (u *usecase) handleAsyncService(ctx context.Context, in *EnqueueInput) (*EnqueueOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	_ = requestID
	lowerService := strings.ToLower(strings.TrimSpace(in.ServiceName))

	// Resolve subject/requester references
	ref, err := u.determineSubjectFromRequester(ctx)
	if err != nil {
		return nil, err
	}

	subjectRef := ref
	requesterRef := ""
	if !strings.EqualFold(ref, string(constvars.ServiceRequestSubjectGuest)) {
		requesterRef = ref
	}

	instantiate := strings.TrimRight(u.cfg.App.BaseUrl, "/") + strings.TrimRight(u.cfg.App.WebhookInstantiateBasePath, "/") + "/" + lowerService

	req := &fhir_dto.CreateServiceRequestInput{
		ResourceType: constvars.ResourceServiceRequest,
		Status:       "active",
		Intent:       "order",
		Subject:      fhir_dto.Reference{Reference: subjectRef},
		InstantiatesUri: []string{
			instantiate,
		},
		AuthoredOn: time.Now().UTC(),
	}
	if requesterRef != "" {
		req.Requester = &fhir_dto.Reference{Reference: requesterRef}
	}

	out, err := u.serviceRequestFhir.CreateServiceRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if out == nil || strings.TrimSpace(out.ID) == "" {
		return nil, exceptions.ErrCreateFHIRResource(nil, constvars.ResourceServiceRequest)
	}

	// Inject/override serviceRequestId in body
	var payload map[string]interface{}
	if len(in.RawJSON) > 0 {
		_ = json.Unmarshal(in.RawJSON, &payload)
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}

	callbackURL := strings.TrimRight(u.cfg.App.BaseUrl, "/") + "/api/v1/callback/service-request"
	payload["url"] = callbackURL
	payload["serviceRequestId"] = out.ID

	newBody, err := json.Marshal(payload)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	// Enqueue message
	msg := webhookqueue.WebhookQueueMessage{
		ID:          uuid.NewString(),
		Method:      in.Method,
		ServiceName: lowerService,
		Body:        json.RawMessage(newBody),
		FailedCount: 0,
	}
	if _, err := u.queue.Enqueue(ctx, &webhookqueue.EnqueueToWebhookServiceQueueInput{Message: msg}); err != nil {
		return nil, err
	}

	return &EnqueueOutput{AsyncServiceResultID: out.ID}, nil
}

// HandleAsyncServiceResult processes callback results for async service requests.
func (u *usecase) HandleAsyncServiceResult(ctx context.Context, in *HandleAsyncServiceResultInput) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	u.log.Info("webhook.usecase.HandleAsyncServiceResult called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_request_id", in.ServiceRequestID),
	)

	// Check authentication - only superadmin allowed
	roles, ok := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	if !ok || !slices.Contains(roles, constvars.KonsulinRoleSuperadmin) {
		return exceptions.ErrAuthInvalidRole(errors.New("unauthorized access"))
	}

	// Validate input
	if err := in.validate(); err != nil {
		return err
	}

	// Search for the ServiceRequest
	searchResults, err := u.serviceRequestFhir.Search(ctx, &fhir_dto.SearchServiceRequestInput{
		ID: in.ServiceRequestID,
	})
	if err != nil {
		u.log.Error("HandleAsyncServiceResult: search failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	// Check result count
	if len(searchResults) == 0 {
		return exceptions.ErrNoDataFHIRResource(nil, constvars.ResourceServiceRequest)
	}
	if len(searchResults) != 1 {
		return exceptions.ErrResultFetchedNotUniqueFhirResource(nil, constvars.ResourceServiceRequest)
	}

	serviceRequest := searchResults[0]

	// Append new note
	newNote := fhir_dto.Annotation{
		Text: in.Result,
		Time: in.Timestamp.Format(time.RFC3339),
	}
	serviceRequest.Note = append(serviceRequest.Note, newNote)

	// Convert to UpdateServiceRequestInput
	updateInput := &fhir_dto.UpdateServiceRequestInput{
		ResourceType:       serviceRequest.ResourceType,
		ID:                 serviceRequest.ID,
		Meta:               serviceRequest.Meta,
		Status:             serviceRequest.Status,
		Intent:             serviceRequest.Intent,
		Subject:            serviceRequest.Subject,
		Requester:          &serviceRequest.Requester,
		OccurrenceDateTime: serviceRequest.OccurrenceDateTime,
		AuthoredOn:         serviceRequest.AuthoredOn,
		InstantiatesUri:    serviceRequest.InstantiatesUri,
		Note:               serviceRequest.Note,
	}

	// Update the ServiceRequest
	_, err = u.serviceRequestFhir.Update(ctx, in.ServiceRequestID, updateInput)
	if err != nil {
		u.log.Error("HandleAsyncServiceResult: update failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// GetAsyncServiceResult retrieves the result of an async service request.
func (u *usecase) GetAsyncServiceResult(ctx context.Context, id string) (*GetAsyncServiceResultOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	u.log.Info("webhook.usecase.GetAsyncServiceResult called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_request_id", id),
	)

	// Search for the ServiceRequest
	searchResults, err := u.serviceRequestFhir.Search(ctx, &fhir_dto.SearchServiceRequestInput{
		ID: id,
	})
	if err != nil {
		u.log.Error("GetAsyncServiceResult: search failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	// Check result count
	if len(searchResults) == 0 {
		return nil, exceptions.ErrNoDataFHIRResource(nil, constvars.ResourceServiceRequest)
	}
	if len(searchResults) != 1 {
		return nil, exceptions.ErrResultFetchedNotUniqueFhirResource(nil, constvars.ResourceServiceRequest)
	}

	serviceRequest := searchResults[0]

	// Concatenate all notes with space separator
	var notesText []string
	for _, note := range serviceRequest.Note {
		if strings.TrimSpace(note.Text) != "" {
			notesText = append(notesText, note.Text)
		}
	}
	concatenatedNotes := strings.Join(notesText, " ")

	// Build output
	output := &GetAsyncServiceResultOutput{
		ResourceType:    serviceRequest.ResourceType,
		Status:          serviceRequest.Status,
		Intent:          serviceRequest.Intent,
		InstantiatesUri: serviceRequest.InstantiatesUri,
		Subject:         serviceRequest.Subject,
		Requester:       serviceRequest.Requester,
		AuthoredOn:      serviceRequest.AuthoredOn,
		Note:            concatenatedNotes,
	}

	u.log.Info("GetAsyncServiceResult succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_request_id", id),
	)
	return output, nil
}
