package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/app/services/shared/webhookqueue"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/go-playground/validator/v10"
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
	// HandleSynchronousWebhookService forwards synchronous webhook services with RBAC and validation.
	HandleSynchronousWebhookService(ctx context.Context, in *HandleSynchronousWebhookServiceInput) (*HandleSynchronousWebhookServiceOutput, error)
}

type usecase struct {
	log                *zap.Logger
	cfg                *config.InternalConfig
	queue              *webhookqueue.Service
	jwtManager         *jwtmanager.JWTManager
	patientFhir        contracts.PatientFhirClient
	practitionerFhir   contracts.PractitionerFhirClient
	personFhir         contracts.PersonFhirClient
	serviceRequestFhir contracts.ServiceRequestFhirClient
	enforcer           *casbin.Enforcer
	syncServiceSet     map[string]struct{}
	httpClient         *http.Client
	failurePolicy      SyncFailurePolicy
}

// NewUsecase creates a new webhook usecase instance.
func NewUsecase(log *zap.Logger, cfg *config.InternalConfig, queue *webhookqueue.Service, jwtMgr *jwtmanager.JWTManager, patient contracts.PatientFhirClient, practitioner contracts.PractitionerFhirClient, person contracts.PersonFhirClient, sr contracts.ServiceRequestFhirClient, enforcer *casbin.Enforcer) Usecase {
	syncSet := make(map[string]struct{})
	for _, s := range cfg.Webhook.SynchronousServiceNames {
		name := strings.ToLower(strings.TrimSpace(s))
		if name != "" {
			syncSet[name] = struct{}{}
		}
	}

	timeout := time.Duration(cfg.Webhook.HTTPTimeoutInSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	policy := SyncFailurePolicyReturnError
	switch strings.ToLower(strings.TrimSpace(cfg.Webhook.SynchronousServiceFailurePolicy)) {
	case string(SyncFailurePolicyEnqueueRequest):
		policy = SyncFailurePolicyEnqueueRequest
	default:
		policy = SyncFailurePolicyReturnError
	}

	return &usecase{
		log:                log,
		cfg:                cfg,
		queue:              queue,
		jwtManager:         jwtMgr,
		patientFhir:        patient,
		practitionerFhir:   practitioner,
		personFhir:         person,
		serviceRequestFhir: sr,
		enforcer:           enforcer,
		syncServiceSet:     syncSet,
		httpClient:         &http.Client{Timeout: timeout},
		failurePolicy:      policy,
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

// SyncFailurePolicy represents fallback behavior for synchronous webhook failures.
type SyncFailurePolicy string

// all known failure policies
const (
	SyncFailurePolicyReturnError    SyncFailurePolicy = "return_error"
	SyncFailurePolicyEnqueueRequest SyncFailurePolicy = "enqueue_request"
)

// synchronousHookPathPrefix is used only to interact with RBAC rules, not
// representing the actual path in the router.
const synchronousHookPathPrefix = "/hook/synchronous"

// HandleSynchronousWebhookServiceInput captures input for synchronous webhook processing.
type HandleSynchronousWebhookServiceInput struct {
	ServiceName string          `validate:"required"`
	Method      string          `validate:"required"`
	RawJSON     json.RawMessage `validate:"required"`
}

// HandleSynchronousWebhookServiceOutput carries upstream response back to caller.
type HandleSynchronousWebhookServiceOutput struct {
	StatusCode int
	Body       []byte
}

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

// HandleSynchronousWebhookService forwards synchronous webhook services with RBAC/body validation.
func (u *usecase) HandleSynchronousWebhookService(ctx context.Context, in *HandleSynchronousWebhookServiceInput) (*HandleSynchronousWebhookServiceOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	u.log.Info("webhook.usecase.HandleSynchronousWebhookService called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_name", in.ServiceName),
		zap.String("method", in.Method),
	)

	if err := validator.New().Struct(in); err != nil {
		return nil, exceptions.ErrInputValidation(err)
	}

	service := strings.ToLower(strings.TrimSpace(in.ServiceName))
	if !u.isSyncService(service) {
		return nil, exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "service is not enabled for synchronous processing", "WEBHOOK_SYNC_SERVICE_NOT_ALLOWED")
	}

	forwarded := ""
	if v := ctx.Value(JWTForwardedFromPaymentServiceHeader); v != nil {
		if s, ok := v.(string); ok {
			forwarded = s
		}
	}
	if err := u.evaluateWebhookAuth(ctx, &evaluateAuthInput{ServiceName: service, ForwardedJWT: forwarded}); err != nil {
		return nil, err
	}

	roles, _ := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	if len(roles) == 0 {
		roles = []string{constvars.KonsulinRoleGuest}
	}
	userIdentifier, _ := ctx.Value(constvars.CONTEXT_UID).(string)

	if err := u.authorizeSynchronous(ctx, roles, in.Method, service); err != nil {
		return nil, err
	}

	if err := u.validateSynchronousBody(ctx, roles, userIdentifier, in.RawJSON); err != nil {
		return nil, err
	}

	out, err := u.forwardSynchronous(ctx, service, in.Method, in.RawJSON)
	if err != nil {
		return u.applySynchronousFailurePolicy(ctx, service, in)
	}

	// when the upstream return non 2xx code, will also apply the fallback policy
	if out.StatusCode < 200 || out.StatusCode >= 300 {
		return u.applySynchronousFailurePolicy(ctx, service, in)
	}

	return out, nil
}

func (u *usecase) isAsyncService(svc string) bool {
	if strings.TrimSpace(svc) == "" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(svc))
	return slices.Contains(u.cfg.Webhook.AsyncServiceNames, target)
}

func (u *usecase) isSyncService(svc string) bool {
	if strings.TrimSpace(svc) == "" {
		return false
	}
	_, ok := u.syncServiceSet[strings.ToLower(strings.TrimSpace(svc))]
	return ok
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

func (u *usecase) authorizeSynchronous(ctx context.Context, roles []string, method, service string) error {
	if u.enforcer == nil {
		return exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, "Not authorized", "WEBHOOK_SYNC_RBAC_DISABLED")
	}
	path := fmt.Sprintf("%s/%s", synchronousHookPathPrefix, service)
	for _, role := range roles {
		normalized := strings.TrimSpace(role)
		if normalized == "" {
			continue
		}
		ok, err := u.enforcer.Enforce(normalized, method, path)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, "Not authorized", "WEBHOOK_SYNC_FORBIDDEN")
}

func (u *usecase) validateSynchronousBody(ctx context.Context, roles []string, userIdentifierId string, raw json.RawMessage) error {
	if containsRole(roles, constvars.KonsulinRoleGuest) || containsRole(roles, constvars.KonsulinRoleSuperadmin) {
		return nil
	}

	email, phone, chatwoot, err := parseRootContactFields(raw)
	if err != nil {
		return err
	}
	if email == "" && phone == "" && chatwoot == "" {
		return nil
	}

	role := selectRoleForValidation(roles)
	if role == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "WEBHOOK_SYNC_UNSUPPORTED_ROLE")
	}

	if strings.TrimSpace(userIdentifierId) == "" || strings.EqualFold(userIdentifierId, "anonymous") {
		return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "WEBHOOK_SYNC_MISSING_UID")
	}

	switch role {
	case constvars.KonsulinRolePractitioner, constvars.KonsulinRoleClinician:
		pracs, err := u.practitionerFhir.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, userIdentifierId)
		if err != nil {
			return err
		}
		if len(pracs) == 0 {
			return exceptions.BuildNewCustomError(nil, constvars.StatusNotFound, "practitioner not found", "WEBHOOK_SYNC_USER_NOT_FOUND")
		}

		prac := pracs[0]

		return validateContactFields(email, phone, chatwoot, prac.GetEmailAddresses(), prac.GetPhoneNumbers(), prac.Identifier, constvars.ResourcePractitioner)
	case constvars.KonsulinRoleClinicAdmin:
		persons, err := u.personFhir.Search(ctx, contracts.PersonSearchInput{
			Identifier: fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, userIdentifierId),
		})
		if err != nil {
			return err
		}
		if len(persons) == 0 {
			return exceptions.BuildNewCustomError(nil, constvars.StatusNotFound, "person not found", "WEBHOOK_SYNC_USER_NOT_FOUND")
		}
		person := persons[0]
		return validateContactFields(email, phone, chatwoot, person.GetEmailAddresses(), person.GetPhoneNumbers(), person.Identifier, constvars.ResourcePerson)
	case constvars.KonsulinRolePatient:
		pats, err := u.patientFhir.FindPatientByIdentifier(ctx, fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, userIdentifierId))
		if err != nil {
			return err
		}
		if len(pats) == 0 {
			return exceptions.BuildNewCustomError(nil, constvars.StatusNotFound, "patient not found", "WEBHOOK_SYNC_USER_NOT_FOUND")
		}

		pat := pats[0]

		return validateContactFields(email, phone, chatwoot, pat.GetEmailAddresses(), pat.GetPhoneNumbers(), pat.Identifier, constvars.ResourcePatient)
	default:
		return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "WEBHOOK_SYNC_UNSUPPORTED_ROLE")
	}
}

func (u *usecase) forwardSynchronous(ctx context.Context, service, method string, body []byte) (*HandleSynchronousWebhookServiceOutput, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimRight(u.cfg.Webhook.URL, "/"), service)
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)

	tokenOut, err := u.jwtManager.CreateToken(ctx, &jwtmanager.CreateTokenInput{Subject: service})
	if err != nil {
		return nil, err
	}
	req.Header.Set(constvars.HeaderAuthorization, "Bearer "+tokenOut.Token)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return &HandleSynchronousWebhookServiceOutput{
		StatusCode: resp.StatusCode,
		Body:       respBody,
	}, nil
}

func (u *usecase) enqueueFallback(ctx context.Context, service, method string, body []byte) error {
	msg := webhookqueue.WebhookQueueMessage{
		ID:          uuid.NewString(),
		Method:      method,
		ServiceName: strings.ToLower(strings.TrimSpace(service)),
		Body:        json.RawMessage(body),
		FailedCount: 0,
	}
	_, err := u.queue.Enqueue(ctx, &webhookqueue.EnqueueToWebhookServiceQueueInput{Message: msg})
	return err
}

func selectRoleForValidation(roles []string) string {
	for _, r := range roles {
		if strings.EqualFold(r, constvars.KonsulinRolePractitioner) || strings.EqualFold(r, constvars.KonsulinRoleClinician) {
			return constvars.KonsulinRolePractitioner
		}
	}
	for _, r := range roles {
		if strings.EqualFold(r, constvars.KonsulinRoleClinicAdmin) {
			return constvars.KonsulinRoleClinicAdmin
		}
	}
	for _, r := range roles {
		if strings.EqualFold(r, constvars.KonsulinRolePatient) {
			return constvars.KonsulinRolePatient
		}
	}
	return ""
}

func containsRole(roles []string, target string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, target) {
			return true
		}
	}
	return false
}

func validateContactFields(email, phone, chatwoot string, emails []string, phones []string, identifiers []fhir_dto.Identifier, resource string) error {
	if email != "" {
		if len(emails) == 0 {
			return exceptions.BuildNewCustomError(nil, constvars.StatusNotFound, fmt.Sprintf("email not found on %s resource", resource), "WEBHOOK_SYNC_CONTACT_NOT_FOUND")
		}
		if !containsCaseInsensitive(emails, email) {
			return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "email does not match authenticated user", "WEBHOOK_SYNC_CONTACT_MISMATCH")
		}
	}

	if phone != "" {
		if len(phones) == 0 {
			return exceptions.BuildNewCustomError(nil, constvars.StatusNotFound, fmt.Sprintf("phone_number not found on %s resource", resource), "WEBHOOK_SYNC_CONTACT_NOT_FOUND")
		}
		if !containsCaseInsensitive(phones, phone) {
			return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "phone_number does not match authenticated user", "WEBHOOK_SYNC_CONTACT_MISMATCH")
		}
	}

	if chatwoot != "" {
		value, found := findIdentifierValue(identifiers, constvars.KonsulinOmnichannelSystemIdentifier)
		if !found {
			return exceptions.BuildNewCustomError(nil, constvars.StatusNotFound, fmt.Sprintf("chatwoot_id not found on %s resource", resource), "WEBHOOK_SYNC_CONTACT_NOT_FOUND")
		}
		if !strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(chatwoot)) {
			return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "chatwoot_id does not match authenticated user", "WEBHOOK_SYNC_CONTACT_MISMATCH")
		}
	}
	return nil
}

func containsCaseInsensitive(list []string, target string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func findIdentifierValue(ids []fhir_dto.Identifier, system string) (string, bool) {
	for _, id := range ids {
		if strings.EqualFold(strings.TrimSpace(id.System), strings.TrimSpace(system)) && strings.TrimSpace(id.Value) != "" {
			return id.Value, true
		}
	}
	return "", false
}

func parseRootContactFields(raw json.RawMessage) (email, phone, chatwoot string, err error) {
	if len(raw) == 0 {
		return "", "", "", nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", "", exceptions.ErrCannotParseJSON(err)
	}
	email = rootString(payload, "email")
	phone = rootString(payload, "phone_number")
	chatwoot = rootString(payload, "chatwoot_id")
	return
}

func rootString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if reflect.TypeOf(v).Kind() == reflect.String {
			return strings.TrimSpace(v.(string))
		}
		if reflect.TypeOf(v).Kind() == reflect.Int {
			return strconv.Itoa(v.(int))
		}
		if reflect.TypeOf(v).Kind() == reflect.Float64 {
			intVal := int(v.(float64))
			return strconv.Itoa(intVal)
		}
	}
	return ""
}

func (u *usecase) applySynchronousFailurePolicy(ctx context.Context, service string, in *HandleSynchronousWebhookServiceInput) (*HandleSynchronousWebhookServiceOutput, error) {
	switch u.failurePolicy {
	case SyncFailurePolicyEnqueueRequest:
		if enqueueErr := u.enqueueFallback(ctx, service, in.Method, in.RawJSON); enqueueErr != nil {
			return nil, enqueueErr
		}
		return &HandleSynchronousWebhookServiceOutput{
			StatusCode: constvars.StatusAccepted,
			Body:       []byte(`{"status":"enqueued"}`),
		}, nil
	default:
		return nil, exceptions.BuildNewCustomError(nil, constvars.StatusInternalServerError, "failed to forward synchronous webhook service", "WEBHOOK_SYNC_FAILURE_POLICY_RETURN_ERROR")
	}
}
