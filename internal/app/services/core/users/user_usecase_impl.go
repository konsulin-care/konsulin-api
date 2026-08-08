package users

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
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type userUsecase struct {
	PatientFhirClient          contracts.PatientFhirClient
	PractitionerFhirClient     contracts.PractitionerFhirClient
	PersonFhirClient           contracts.PersonFhirClient
	PractitionerRoleFhirClient contracts.PractitionerRoleFhirClient
	OrganizationFhirClient     contracts.OrganizationFhirClient
	RedisRepository            contracts.RedisRepository
	InternalConfig             *config.InternalConfig
	Log                        *zap.Logger
	LockerService              contracts.LockerService
	JWTTokenManager            *jwtmanager.JWTManager

	// webhookForwardFn, when set, bypasses the HTTP loopback for synchronous webhook forwarding.
	// Intended for in-process callers that are already trusted.
	webhookForwardFn func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error)
}

// SetWebhookForwarder sets the in-process forwarder for synchronous webhook calls.
// When set, callWebhookSvcKonsulinOmnichannel calls this function directly instead
// of making an HTTP loopback to the backend's own webhook endpoint.
func (uc *userUsecase) SetWebhookForwarder(fn func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error)) {
	uc.webhookForwardFn = fn
}

var (
	userUsecaseInstance     contracts.UserFHIRInitializer
	onceUserFHIRInitializer sync.Once
)

func NewUserFHIRInitializer(
	patientFhirClient contracts.PatientFhirClient,
	practitionerFhirClient contracts.PractitionerFhirClient,
	personFhirClient contracts.PersonFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	organizationFhirClient contracts.OrganizationFhirClient,
	redisRepository contracts.RedisRepository,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
	lockerService contracts.LockerService,
	jwtManager *jwtmanager.JWTManager,
) contracts.UserFHIRInitializer {
	onceUserFHIRInitializer.Do(func() {
		instance := &userUsecase{
			PatientFhirClient:          patientFhirClient,
			PractitionerFhirClient:     practitionerFhirClient,
			PersonFhirClient:           personFhirClient,
			PractitionerRoleFhirClient: practitionerRoleFhirClient,
			OrganizationFhirClient:     organizationFhirClient,
			RedisRepository:            redisRepository,
			InternalConfig:             internalConfig,
			Log:                        logger,
			LockerService:              lockerService,
			JWTTokenManager:            jwtManager,
		}
		userUsecaseInstance = instance
	})
	return userUsecaseInstance
}

func (uc *userUsecase) InitializeNewUserFHIRResources(ctx context.Context, input *contracts.InitializeNewUserFHIRResourcesInput) (*contracts.InitializeNewUserFHIRResourcesOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, exceptions.ErrInvalidFormat(err, "email_or_phone")
	}

	output := &contracts.InitializeNewUserFHIRResourcesOutput{}

	for _, resource := range input.Resources() {
		switch resource {
		case constvars.ResourcePractitioner:
			practitioner, err := uc.createPractitionerIfNotExists(ctx, input.Email, input.Phone, input.SuperTokenUserID)
			if err != nil {
				return nil, err
			}
			output.PractitionerID = practitioner.ID
		case constvars.ResourcePatient:
			patient, err := uc.createPatientIfNotExists(ctx, input.Email, input.Phone, input.SuperTokenUserID)
			if err != nil {
				return nil, err
			}
			output.PatientID = patient.ID
		case constvars.ResourcePerson:
			person, err := uc.createPersonIfNotExists(ctx, input.Email, input.Phone, input.SuperTokenUserID)
			if err != nil {
				return nil, err
			}
			output.PersonID = person.ID
		}
	}
	return output, nil
}

// identifierScanResult holds the result of scanning identifiers for supertoken and Chatwoot IDs.
type identifierScanResult struct {
	foundSupertoken      bool
	foundSupertokenIdx   int
	supertokenExactMatch bool
	foundChatwoot        bool
	foundChatwootIdx     int
	chatwootExactMatch   bool
}

// scanIdentifiers iterates over identifiers and records match status for both
// supertoken and Chatwoot omnichannel identifier systems.
// Returns -1 indices when not found.
func scanIdentifiers(identifiers []fhir_dto.Identifier, superTokenUserID, chatwootID string) identifierScanResult {
	result := identifierScanResult{
		foundSupertokenIdx: -1,
		foundChatwootIdx:   -1,
	}
	for idx, identifier := range identifiers {
		if identifier.System == constvars.FhirSupertokenSystemIdentifier {
			result.foundSupertoken = true
			result.foundSupertokenIdx = idx
			result.supertokenExactMatch = identifier.Value == superTokenUserID
		}
		if identifier.System == constvars.KonsulinOmnichannelSystemIdentifier {
			result.foundChatwoot = true
			result.foundChatwootIdx = idx
			result.chatwootExactMatch = identifier.Value == chatwootID
		}
	}
	return result
}

// callChatwootWithFallback calls the omnichannel webhook to create/update a Chatwoot contact.
// If email and phone are both empty, it returns immediately with no error.
// If the call fails, the error is logged and returned for callers to decide how to handle it.
func (uc *userUsecase) callChatwootWithFallback(ctx context.Context, email, phone, username string) (callWebhookSvcKonsulinOmnichannelOutput, error) {
	if strings.TrimSpace(email) == "" && strings.TrimSpace(phone) == "" {
		return callWebhookSvcKonsulinOmnichannelOutput{}, nil
	}
	result, err := uc.callWebhookSvcKonsulinOmnichannel(ctx, callWebhookSvcKonsulinOmnichannelInput{
		Email:    email,
		Username: username,
		Phone:    phone,
	})
	if err != nil {
		uc.Log.Error("userUsecase.callChatwootWithFallback error calling webhook svc konsulin omnichannel",
			zap.Error(err),
		)
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}
	return result, nil
}

// ensurePractitionerIdentifiers updates the practitioner's identifiers with the
// supertoken user ID and Chatwoot contact ID if they differ from what's stored.
// Returns the (possibly updated) practitioner.
//
//nolint:dupl
func (uc *userUsecase) ensurePractitionerIdentifiers(ctx context.Context, practitioner *fhir_dto.Practitioner, email, phone, superTokenUserID string) (*fhir_dto.Practitioner, error) {
	userChatwootContact, chatwootCallErr := uc.callChatwootWithFallback(ctx, email, phone, practitioner.FullName())
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

	scanResult := scanIdentifiers(practitioner.Identifier, superTokenUserID, chatwootID)
	mustUpdate := false

	if superTokenUserID != "" {
		if scanResult.foundSupertoken && !scanResult.supertokenExactMatch {
			mustUpdate = true
			practitioner.Identifier[scanResult.foundSupertokenIdx] = fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			}
		}
		if !scanResult.foundSupertoken {
			mustUpdate = true
			practitioner.Identifier = append(practitioner.Identifier, fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			})
		}
	}

	if chatwootCallErr == nil && userChatwootContact.ChatwootID != 0 {
		if !scanResult.foundChatwoot {
			mustUpdate = true
			practitioner.Identifier = append(practitioner.Identifier, fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			})
		}
		if scanResult.foundChatwoot && !scanResult.chatwootExactMatch {
			mustUpdate = true
			practitioner.Identifier[scanResult.foundChatwootIdx] = fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			}
		}
	}

	if mustUpdate {
		return uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitioner)
	}
	return practitioner, nil
}

// lookupPractitioner searches for an existing Practitioner by email, phone, or supertoken identifier.
func (uc *userUsecase) lookupPractitioner(ctx context.Context, email, phone, superTokenUserID string) ([]fhir_dto.Practitioner, error) {
	if strings.TrimSpace(email) != "" {
		return uc.PractitionerFhirClient.FindPractitionerByEmail(ctx, email)
	}
	if strings.TrimSpace(phone) != "" {
		return uc.PractitionerFhirClient.FindPractitionerByPhone(ctx, phone)
	}
	if strings.TrimSpace(superTokenUserID) != "" {
		return uc.PractitionerFhirClient.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, superTokenUserID)
	}
	return nil, nil
}

// lookupPatient searches for an existing Patient by email, phone, or supertoken identifier.
func (uc *userUsecase) lookupPatient(ctx context.Context, email, phone, superTokenUserID string) ([]fhir_dto.Patient, error) {
	if strings.TrimSpace(email) != "" {
		return uc.PatientFhirClient.FindPatientByEmail(ctx, email)
	}
	if strings.TrimSpace(phone) != "" {
		return uc.PatientFhirClient.FindPatientByPhone(ctx, phone)
	}
	if strings.TrimSpace(superTokenUserID) != "" {
		return uc.PatientFhirClient.FindPatientByIdentifier(
			ctx,
			fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, superTokenUserID),
		)
	}
	return nil, nil
}

// lookupPerson searches for an existing Person by email, phone, or supertoken identifier.
func (uc *userUsecase) lookupPerson(ctx context.Context, email, phone, superTokenUserID string) ([]fhir_dto.Person, error) {
	if strings.TrimSpace(email) != "" {
		return uc.PersonFhirClient.FindPersonByEmail(ctx, email)
	}
	if strings.TrimSpace(phone) != "" {
		return uc.PersonFhirClient.FindPersonByPhone(ctx, phone)
	}
	if strings.TrimSpace(superTokenUserID) != "" {
		return uc.PersonFhirClient.Search(ctx, contracts.PersonSearchInput{
			Identifier: fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, superTokenUserID),
		})
	}
	return nil, nil
}

// ensurePatientIdentifiers updates the patient's identifiers with the
// supertoken user ID and Chatwoot contact ID if they differ from what's stored.
//
//nolint:dupl
func (uc *userUsecase) ensurePatientIdentifiers(ctx context.Context, patient *fhir_dto.Patient, email, phone, superTokenUserID string) (*fhir_dto.Patient, error) {
	userChatwootContact, chatwootCallErr := uc.callChatwootWithFallback(ctx, email, phone, patient.FullName())
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

	scanResult := scanIdentifiers(patient.Identifier, superTokenUserID, chatwootID)
	mustUpdate := false

	if superTokenUserID != "" {
		if scanResult.foundSupertoken && !scanResult.supertokenExactMatch {
			mustUpdate = true
			patient.Identifier[scanResult.foundSupertokenIdx] = fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			}
		}
		if !scanResult.foundSupertoken {
			mustUpdate = true
			patient.Identifier = append(patient.Identifier, fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			})
		}
	}

	if chatwootCallErr == nil && userChatwootContact.ChatwootID != 0 {
		if !scanResult.foundChatwoot {
			mustUpdate = true
			patient.Identifier = append(patient.Identifier, fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			})
		}
		if scanResult.foundChatwoot && !scanResult.chatwootExactMatch {
			mustUpdate = true
			patient.Identifier[scanResult.foundChatwootIdx] = fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			}
		}
	}

	if mustUpdate {
		return uc.PatientFhirClient.UpdatePatient(ctx, patient)
	}
	return patient, nil
}

// ensurePersonIdentifiers updates the person's identifiers with the
// supertoken user ID and Chatwoot contact ID if they differ from what's stored.
//
//nolint:dupl
func (uc *userUsecase) ensurePersonIdentifiers(ctx context.Context, person *fhir_dto.Person, email, phone, superTokenUserID string) (*fhir_dto.Person, error) {
	userChatwootContact, chatwootCallErr := uc.callChatwootWithFallback(ctx, email, phone, person.FullName())
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

	scanResult := scanIdentifiers(person.Identifier, superTokenUserID, chatwootID)
	mustUpdate := false

	if superTokenUserID != "" {
		if scanResult.foundSupertoken && !scanResult.supertokenExactMatch {
			mustUpdate = true
			person.Identifier[scanResult.foundSupertokenIdx] = fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			}
		}
		if !scanResult.foundSupertoken {
			mustUpdate = true
			person.Identifier = append(person.Identifier, fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			})
		}
	}

	if chatwootCallErr == nil && userChatwootContact.ChatwootID != 0 {
		if !scanResult.foundChatwoot {
			mustUpdate = true
			person.Identifier = append(person.Identifier, fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			})
		}
		if scanResult.foundChatwoot && !scanResult.chatwootExactMatch {
			mustUpdate = true
			person.Identifier[scanResult.foundChatwootIdx] = fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			}
		}
	}

	if mustUpdate {
		return uc.PersonFhirClient.Update(ctx, person)
	}
	return person, nil
}

// createNewPractitioner creates a new Practitioner FHIR resource.
//
//nolint:dupl
func (uc *userUsecase) createNewPractitioner(ctx context.Context, email, phone, superTokenUserID string) (*fhir_dto.Practitioner, error) {
	if superTokenUserID == "" {
		return nil, exceptions.ErrInvalidFormat(nil, "superTokenUserID is required for creating a user with Practitioner role")
	}

	userChatwootContact, chatwootErr := uc.callChatwootWithFallback(ctx, email, phone, "")
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)
	telecom := buildContactPoints(email, phone)

	newPractitionerInput := &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		Active:       true,
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: superTokenUserID},
		},
		Telecom: telecom,
	}

	if chatwootErr == nil && userChatwootContact.ChatwootID != 0 {
		newPractitionerInput.Identifier = append(newPractitionerInput.Identifier, fhir_dto.Identifier{
			System: constvars.KonsulinOmnichannelSystemIdentifier,
			Value:  chatwootID,
		})
	}

	return uc.PractitionerFhirClient.CreatePractitioner(ctx, newPractitionerInput)
}

// createNewPatient creates a new Patient FHIR resource.
//
//nolint:dupl
func (uc *userUsecase) createNewPatient(ctx context.Context, email, phone, superTokenUserID string) (*fhir_dto.Patient, error) {
	userChatwootContact, chatwootErr := uc.callChatwootWithFallback(ctx, email, phone, "")
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)
	telecom := buildContactPoints(email, phone)

	newPatientInput := &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		Active:       true,
		Identifier:   []fhir_dto.Identifier{},
		Telecom:      telecom,
	}

	if superTokenUserID != "" {
		newPatientInput.Identifier = append(newPatientInput.Identifier, fhir_dto.Identifier{
			System: constvars.FhirSupertokenSystemIdentifier,
			Value:  superTokenUserID,
		})
	}

	if chatwootErr == nil && userChatwootContact.ChatwootID != 0 {
		newPatientInput.Identifier = append(newPatientInput.Identifier, fhir_dto.Identifier{
			System: constvars.KonsulinOmnichannelSystemIdentifier,
			Value:  chatwootID,
		})
	}

	return uc.PatientFhirClient.CreatePatient(ctx, newPatientInput)
}

// createNewPerson creates a new Person FHIR resource.
//
//nolint:dupl
func (uc *userUsecase) createNewPerson(ctx context.Context, email, phone, superTokenUserID string) (*fhir_dto.Person, error) {
	userChatwootContact, chatwootErr := uc.callChatwootWithFallback(ctx, email, phone, "")
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)
	telecom := buildContactPoints(email, phone)

	newPersonInput := &fhir_dto.Person{
		ResourceType: constvars.ResourcePerson,
		Active:       true,
		Identifier:   []fhir_dto.Identifier{},
		Telecom:      telecom,
	}

	if superTokenUserID != "" {
		newPersonInput.Identifier = append(newPersonInput.Identifier, fhir_dto.Identifier{
			System: constvars.FhirSupertokenSystemIdentifier,
			Value:  superTokenUserID,
		})
	}

	if chatwootErr == nil && userChatwootContact.ChatwootID != 0 {
		newPersonInput.Identifier = append(newPersonInput.Identifier, fhir_dto.Identifier{
			System: constvars.KonsulinOmnichannelSystemIdentifier,
			Value:  chatwootID,
		})
	}

	return uc.PersonFhirClient.Create(ctx, newPersonInput)
}

//nolint:dupl
func (uc *userUsecase) createPractitionerIfNotExists(ctx context.Context, email string, phone string, superTokenUserID string) (*fhir_dto.Practitioner, error) {
	practitioners, err := uc.lookupPractitioner(ctx, email, phone, superTokenUserID)
	if err != nil {
		return nil, err
	}
	if len(practitioners) > 0 {
		return uc.ensurePractitionerIdentifiers(ctx, &practitioners[0], email, phone, superTokenUserID)
	}
	return uc.createNewPractitioner(ctx, email, phone, superTokenUserID)
}

//nolint:dupl
func (uc *userUsecase) createPatientIfNotExists(ctx context.Context, email string, phone string, superTokenUserID string) (*fhir_dto.Patient, error) {
	patients, err := uc.lookupPatient(ctx, email, phone, superTokenUserID)
	if err != nil {
		return nil, err
	}
	if len(patients) > 0 {
		return uc.ensurePatientIdentifiers(ctx, &patients[0], email, phone, superTokenUserID)
	}
	return uc.createNewPatient(ctx, email, phone, superTokenUserID)
}

//nolint:dupl
func (uc *userUsecase) createPersonIfNotExists(ctx context.Context, email string, phone string, superTokenUserID string) (*fhir_dto.Person, error) {
	persons, err := uc.lookupPerson(ctx, email, phone, superTokenUserID)
	if err != nil {
		return nil, err
	}
	if len(persons) > 0 {
		return uc.ensurePersonIdentifiers(ctx, &persons[0], email, phone, superTokenUserID)
	}
	return uc.createNewPerson(ctx, email, phone, superTokenUserID)
}

type callWebhookSvcKonsulinOmnichannelOutput struct {
	ChatwootID  int    `json:"chatwoot_id"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
}

type callWebhookSvcKonsulinOmnichannelInput struct {
	Email    string
	Username string
	Phone    string
}

type callWebhookSvcKonsulinOmnichannelRawOutput struct {
	ChatwootID  int     `json:"chatwoot_id"`
	Email       string  `json:"email"`
	PhoneNumber *string `json:"phoneNumber"`
}

func (uc *userUsecase) callWebhookSvcKonsulinOmnichannel(ctx context.Context, input callWebhookSvcKonsulinOmnichannelInput) (callWebhookSvcKonsulinOmnichannelOutput, error) {
	lastUsername := input.Username
	if lastUsername == "" {
		if strings.TrimSpace(input.Email) != "" {
			lastUsername = strings.Split(input.Email, "@")[0]
		}
	}

	// The rest of the system stores phone without a leading '+', but the upstream expects E.164 with '+'.
	// Keep this detail internal so callers don't have to know about it.
	phoneE164 := utils.FormatE164WithPlus(input.Phone)

	body := struct {
		Email string `json:"email,omitempty"`
		Name  string `json:"name"`
		Phone string `json:"phoneNumber,omitempty"`
	}{
		Email: input.Email,
		Name:  lastUsername,
		Phone: phoneE164,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}

	// Use in-process forwarder when available (skips HTTP loopback, JWT creation, auth).
	if uc.webhookForwardFn != nil {
		outStatusCode, respBody, err := uc.webhookForwardFn(ctx, "modify-profile", http.MethodPost, bodyBytes, "application/json")
		if err != nil {
			return callWebhookSvcKonsulinOmnichannelOutput{}, err
		}
		if outStatusCode != http.StatusOK {
			return callWebhookSvcKonsulinOmnichannelOutput{}, errors.New("failed to call webhook svc konsulin omnichannel")
		}

		return parseOmnichannelResponse(respBody)
	}

	tokenOut, err := uc.JWTTokenManager.CreateToken(
		ctx,
		&jwtmanager.CreateTokenInput{
			Subject: constvars.KonsulinOmnichannelSystemIdentifier,
		},
	)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}

	url := fmt.Sprintf(
		"%s/%s/synchronous/modify-profile",
		strings.TrimRight(uc.InternalConfig.App.BaseUrl, "/"),
		strings.Trim(uc.InternalConfig.App.WebhookInstantiateBasePath, "/"),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}

	req.Header.Set(constvars.HeaderAuthorization, "Bearer "+tokenOut.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: time.Duration(uc.InternalConfig.Webhook.HTTPTimeoutInSeconds) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return callWebhookSvcKonsulinOmnichannelOutput{}, errors.New("failed to call webhook svc konsulin omnichannel")
	}

	bodyBytesResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}

	return parseOmnichannelResponse(bodyBytesResp)
}

// parseOmnichannelResponse parses the raw response body from the omnichannel webhook
// into a structured output. It handles nil phone number fields safely.
func parseOmnichannelResponse(body []byte) (callWebhookSvcKonsulinOmnichannelOutput, error) {
	var rawOutputs []callWebhookSvcKonsulinOmnichannelRawOutput
	if err := json.Unmarshal(body, &rawOutputs); err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}
	if len(rawOutputs) == 0 {
		return callWebhookSvcKonsulinOmnichannelOutput{}, errors.New("webhook svc konsulin omnichannel returned empty response")
	}

	raw := rawOutputs[0]
	output := callWebhookSvcKonsulinOmnichannelOutput{
		ChatwootID:  raw.ChatwootID,
		Email:       raw.Email,
		PhoneNumber: "",
	}

	// the upstream server might omit the phone number or assigning null to it
	// this was made to ensure no nil pointer dereference happen when
	// the downstream code try to access the phone number
	if raw.PhoneNumber != nil {
		output.PhoneNumber = *raw.PhoneNumber
	}
	return output, nil
}

// buildContactPoints builds a slice of FHIR ContactPoint from email and phone.
// Non-empty values are included; empty values are skipped.
func buildContactPoints(email, phone string) []fhir_dto.ContactPoint {
	var telecom []fhir_dto.ContactPoint
	if strings.TrimSpace(email) != "" {
		telecom = append(telecom, fhir_dto.ContactPoint{
			System: fhir_dto.ContactPointSystemEmail,
			Value:  email,
			Use:    constvars.FhirAddressUseWork,
		})
	}
	if strings.TrimSpace(phone) != "" {
		telecom = append(telecom, fhir_dto.ContactPoint{
			System: fhir_dto.ContactPointSystemPhone,
			Value:  phone,
			Use:    constvars.FhirAddressUseWork,
		})
	}
	return telecom
}
