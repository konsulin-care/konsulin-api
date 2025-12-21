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
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
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
	UserRepository             contracts.UserRepository
	PatientFhirClient          contracts.PatientFhirClient
	PractitionerFhirClient     contracts.PractitionerFhirClient
	PersonFhirClient           contracts.PersonFhirClient
	PractitionerRoleFhirClient contracts.PractitionerRoleFhirClient
	OrganizationFhirClient     contracts.OrganizationFhirClient
	RedisRepository            contracts.RedisRepository
	SessionService             contracts.SessionService
	InternalConfig             *config.InternalConfig
	Log                        *zap.Logger
	LockerService              contracts.LockerService
	JWTTokenManager            *jwtmanager.JWTManager
}

var (
	userUsecaseInstance contracts.UserUsecase
	onceUserUsecase     sync.Once
)

func NewUserUsecase(
	userMongoRepository contracts.UserRepository,
	patientFhirClient contracts.PatientFhirClient,
	practitionerFhirClient contracts.PractitionerFhirClient,
	personFhirClient contracts.PersonFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	organizationFhirClient contracts.OrganizationFhirClient,
	redisRepository contracts.RedisRepository,
	sessionService contracts.SessionService,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
	lockerService contracts.LockerService,
	jwtManager *jwtmanager.JWTManager,
) contracts.UserUsecase {
	onceUserUsecase.Do(func() {
		instance := &userUsecase{
			UserRepository:             userMongoRepository,
			PatientFhirClient:          patientFhirClient,
			PractitionerFhirClient:     practitionerFhirClient,
			PersonFhirClient:           personFhirClient,
			PractitionerRoleFhirClient: practitionerRoleFhirClient,
			OrganizationFhirClient:     organizationFhirClient,
			RedisRepository:            redisRepository,
			SessionService:             sessionService,
			InternalConfig:             internalConfig,
			Log:                        logger,
			LockerService:              lockerService,
			JWTTokenManager:            jwtManager,
		}
		userUsecaseInstance = instance
	})
	return userUsecaseInstance
}

func (uc *userUsecase) GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error) {
	start := time.Now()
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Debug("User profile retrieval started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOperationKey, "get_user_profile"),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("Failed to parse session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "session parsing"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		return nil, err
	}

	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		uc.Log.Error("Failed to fetch user by ID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, session.UserID),
			zap.String(constvars.LoggingErrorTypeKey, "database query"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		return nil, err
	}

	if existingUser == nil {
		uc.Log.Error("User not found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, session.UserID),
			zap.String(constvars.LoggingErrorTypeKey, "user not found"),
		)
		return nil, exceptions.ErrUserNotExist(nil)
	}

	

	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		uc.Log.Debug("Processing practitioner profile",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, session.UserID),
		)
		return uc.getPractitionerProfile(ctx, session, "")
	case constvars.RoleTypePatient:
		uc.Log.Debug("Processing patient profile",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, session.UserID),
		)
		return uc.getPatientProfile(ctx, session, "")
	default:
		uc.Log.Error("Invalid role type",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("role_name", session.RoleName),
			zap.String(constvars.LoggingErrorTypeKey, "invalid role"),
		)
		return nil, exceptions.ErrInvalidRoleType(nil)
	}
}

func (uc *userUsecase) UpdateUserProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.UpdateUserProfileBySession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("userUsecase.UpdateUserProfileBySession error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if session.Email != request.Email {
		uc.Log.Info("userUsecase.UpdateUserProfileBySession email change detected; checking for existence",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		existingUser, err := uc.UserRepository.FindByEmail(ctx, request.Email)
		if err != nil {
			uc.Log.Error("userUsecase.UpdateUserProfileBySession error checking email existence",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		if existingUser != nil {
			uc.Log.Error("userUsecase.UpdateUserProfileBySession email already exists",
				zap.String(constvars.LoggingRequestIDKey, requestID),
			)
			return nil, exceptions.ErrEmailAlreadyExist(nil)
		}
	}


	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		uc.Log.Error("userUsecase.UpdateUserProfileBySession error fetching user by ID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, session.UserID),
			zap.Error(err),
		)
		return nil, err
	}

	if existingUser == nil {
		uc.Log.Error("userUsecase.UpdateUserProfileBySession user does not exist",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, session.UserID),
		)
		return nil, exceptions.ErrUserNotExist(nil)
	}

	existingUser.SetDataForUpdateProfile(request)
	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		uc.Log.Error("userUsecase.UpdateUserProfileBySession error updating user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("userUsecase.UpdateUserProfileBySession user updated successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, existingUser.ID),
	)

	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		uc.Log.Info("userUsecase.UpdateUserProfileBySession updating practitioner profile",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return uc.updatePractitionerFhirProfile(ctx, existingUser, session, request)
	case constvars.RoleTypePatient:
		uc.Log.Info("userUsecase.UpdateUserProfileBySession updating patient profile",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return uc.updatePatientFhirProfile(ctx, existingUser, session, request)
	default:
		uc.Log.Error("userUsecase.UpdateUserProfileBySession invalid role type",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrInvalidRoleType(nil)
	}
}

func (uc *userUsecase) DeleteUserBySession(ctx context.Context, sessionData string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.DeleteUserBySession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionDataKey, sessionData),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("userUsecase.DeleteUserBySession error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	err = uc.UserRepository.DeleteByID(ctx, session.UserID)
	if err != nil {
		uc.Log.Error("userUsecase.DeleteUserBySession error deleting user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, session.UserID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("userUsecase.DeleteUserBySession user deleted",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, session.UserID),
	)

	err = uc.RedisRepository.Delete(ctx, session.SessionID)
	if err != nil {
		uc.Log.Error("userUsecase.DeleteUserBySession error deleting session from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingSessionIDKey, session.SessionID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("userUsecase.DeleteUserBySession session deleted from Redis",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionIDKey, session.SessionID),
	)
	return nil
}

func (uc *userUsecase) DeactivateUserBySession(ctx context.Context, sessionData string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.DeactivateUserBySession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionDataKey, sessionData),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("userUsecase.DeactivateUserBySession error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		uc.Log.Error("userUsecase.DeactivateUserBySession error fetching user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, session.UserID),
			zap.Error(err),
		)
		return err
	}

	existingUser.SetDeletedAt()
	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		uc.Log.Error("userUsecase.DeactivateUserBySession error updating user for deactivation",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("userUsecase.DeactivateUserBySession user deactivated",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, existingUser.ID),
	)

	err = uc.RedisRepository.Delete(ctx, session.SessionID)
	if err != nil {
		uc.Log.Error("userUsecase.DeactivateUserBySession error deleting session from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingSessionIDKey, session.SessionID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("userUsecase.DeactivateUserBySession session deleted from Redis",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionIDKey, session.SessionID),
	)

	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		uc.Log.Info("userUsecase.DeactivateUserBySession deactivating practitioner FHIR data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return uc.deactivatePractitionerFhirData(ctx, existingUser)
	case constvars.RoleTypePatient:
		uc.Log.Info("userUsecase.DeactivateUserBySession deactivating patient FHIR data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return uc.deactivatePatientFhirData(ctx, existingUser)
	default:
		uc.Log.Error("userUsecase.DeactivateUserBySession invalid role type",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.ErrInvalidRoleType(nil)
	}
}

func (uc *userUsecase) InitializeNewUserFHIRResources(ctx context.Context, input *contracts.InitializeNewUserFHIRResourcesInput) (*contracts.InitializeNewUserFHIRResourcesOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, exceptions.ErrInvalidFormat(err, "email")
	}

	output := &contracts.InitializeNewUserFHIRResourcesOutput{}

	for _, resource := range input.Resources() {
		switch resource {
		case constvars.ResourcePractitioner:
			practitioner, err := uc.createPractitionerIfNotExists(ctx, input.Email, input.SuperTokenUserID)
			if err != nil {
				return nil, err
			}
			output.PractitionerID = practitioner.ID
		case constvars.ResourcePatient:
			patient, err := uc.createPatientIfNotExists(ctx, input.Email, input.SuperTokenUserID)
			if err != nil {
				return nil, err
			}
			output.PatientID = patient.ID
		case constvars.ResourcePerson:
			person, err := uc.createPersonIfNotExists(ctx, input.Email, input.SuperTokenUserID)
			if err != nil {
				return nil, err
			}
			output.PersonID = person.ID
		}
	}
	return output, nil
}

func (uc *userUsecase) deactivatePractitionerFhirData(ctx context.Context, user *models.User) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.deactivatePractitionerFhirData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, user.ID),
	)

	practitionerFhirRequest := user.ConvertToPractitionerFhirDeactivationRequest()

	_, err := uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitionerFhirRequest)
	if err != nil {
		uc.Log.Error("userUsecase.deactivatePractitionerFhirData error updating practitioner FHIR resource",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("userUsecase.deactivatePractitionerFhirData succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *userUsecase) deactivatePatientFhirData(ctx context.Context, user *models.User) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.deactivatePatientFhirData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, user.ID),
	)

	patientFhirRequest := user.ConvertToPatientFhirDeactivationRequest()

	_, err := uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
	if err != nil {
		uc.Log.Error("userUsecase.deactivatePatientFhirData error updating patient FHIR resource",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("userUsecase.deactivatePatientFhirData succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *userUsecase) createPractitionerIfNotExists(ctx context.Context, email string, superTokenUserID string) (*fhir_dto.Practitioner, error) {
	practitioners, err := uc.PractitionerFhirClient.FindPractitionerByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if len(practitioners) > 0 {
		practitioner := practitioners[0]

		// if the chatwoot call fails, we don't need to update the practitioner identifier with the chatwoot ID
		// but the process must continue, just not update the practitioner identifier with the chatwoot ID
		userChatwootContact, chatwootCallErr := uc.callWebhookSvcKonsulinOmnichannel(ctx, email, practitioner.FullName())
		if chatwootCallErr != nil {
			uc.Log.Error("userUsecase.createPractitionerIfNotExists error calling webhook svc konsulin omnichannel",
				zap.Error(chatwootCallErr),
			)
		}

		chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

		mustUpdatePractitioner := false

		foundSupertokenSysId := false
		foundSupertokenSysIdOnIdx := -1
		supertokenSysIdExactMatch := false

		foundChatwootID := false
		foundChatwootIDOnIdx := -1
		chatwootIDExactMatch := false

		for idx, identifier := range practitioner.Identifier {
			if identifier.System == constvars.FhirSupertokenSystemIdentifier {
				foundSupertokenSysId = true
				foundSupertokenSysIdOnIdx = idx

				if identifier.Value == superTokenUserID {
					supertokenSysIdExactMatch = true
				}
			}

			if identifier.System == constvars.KonsulinOmnichannelSystemIdentifier {
				foundChatwootID = true
				foundChatwootIDOnIdx = idx

				if identifier.Value == chatwootID {
					chatwootIDExactMatch = true
				}
			}
		}

		if superTokenUserID != "" {
			if foundSupertokenSysId && !supertokenSysIdExactMatch {
				mustUpdatePractitioner = true
				practitioner.Identifier[foundSupertokenSysIdOnIdx] = fhir_dto.Identifier{
					System: constvars.FhirSupertokenSystemIdentifier,
					Value:  superTokenUserID,
				}
			}

			if !foundSupertokenSysId {
				mustUpdatePractitioner = true
				practitioner.Identifier = append(practitioner.Identifier, fhir_dto.Identifier{
					System: constvars.FhirSupertokenSystemIdentifier,
					Value:  superTokenUserID,
				})
			}
		}

		// only attempt to update the practitioner identifier with the chatwoot ID if the chatwoot call was successful
		// and the chatwoot ID is not 0 (which can means that the user is not yet added to the chatwoot workspace or the API call failed)
		if chatwootCallErr == nil && userChatwootContact.ChatwootID != 0 {
			if !foundChatwootID {
				mustUpdatePractitioner = true
				practitioner.Identifier = append(practitioner.Identifier, fhir_dto.Identifier{
					System: constvars.KonsulinOmnichannelSystemIdentifier,
					Value:  chatwootID,
				})
			}

			if foundChatwootID && !chatwootIDExactMatch {
				mustUpdatePractitioner = true
				practitioner.Identifier[foundChatwootIDOnIdx] = fhir_dto.Identifier{
					System: constvars.KonsulinOmnichannelSystemIdentifier,
					Value:  chatwootID,
				}
			}
		}

		if mustUpdatePractitioner {
			updatedPractitioner, err := uc.PractitionerFhirClient.UpdatePractitioner(ctx, &practitioner)
			if err != nil {
				return nil, err
			}
			return updatedPractitioner, nil
		}

		return &practitioner, nil
	}

	if superTokenUserID == "" {
		return nil, exceptions.ErrInvalidFormat(nil, "superTokenUserID")
	}

	userChatwootContact, chatwootErr := uc.callWebhookSvcKonsulinOmnichannel(ctx, email, "")
	if chatwootErr != nil {
		// log the error but continue the process
		uc.Log.Error("userUsecase.createPractitionerIfNotExists error calling webhook svc konsulin omnichannel",
			zap.Error(chatwootErr),
		)
	}

	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

	newPractitionerInput := &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		Active:       true,
		Identifier: []fhir_dto.Identifier{
			{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			},
		},
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemEmail,
				Value:  email,
				Use:    "work",
			},
		},
	}

	if chatwootErr == nil && userChatwootContact.ChatwootID != 0 {
		newPractitionerInput.Identifier = append(newPractitionerInput.Identifier, fhir_dto.Identifier{
			System: constvars.KonsulinOmnichannelSystemIdentifier,
			Value:  chatwootID,
		})
	}

	newPractitioner, err := uc.PractitionerFhirClient.CreatePractitioner(ctx, newPractitionerInput)
	if err != nil {
		return nil, err
	}

	return newPractitioner, nil
}

func (uc *userUsecase) createPatientIfNotExists(ctx context.Context, email string, superTokenUserID string) (*fhir_dto.Patient, error) {
	patients, err := uc.PatientFhirClient.FindPatientByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if len(patients) > 0 {
		patient := patients[0]

		// if the chatwoot call fails, we don't need to update the patient identifier with the chatwoot ID
		// but the process must continue, just not update the patient identifier with the chatwoot ID
		userChatwootContact, chatwootCallErr := uc.callWebhookSvcKonsulinOmnichannel(ctx, email, patient.FullName())
		if chatwootCallErr != nil {
			// log the error but continue the process
			uc.Log.Error("userUsecase.createPatientIfNotExists error calling webhook svc konsulin omnichannel",
				zap.Error(chatwootCallErr),
			)
		}

		chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

		mustUpdatePatient := false

		foundSupertokenSysId := false
		foundSupertokenSysIdOnIdx := -1
		supertokenSysIdExactMatch := false

		foundChatwootID := false
		foundChatwootIDOnIdx := -1
		chatwootIDExactMatch := false

		for idx, identifier := range patient.Identifier {
			if identifier.System == constvars.FhirSupertokenSystemIdentifier {
				foundSupertokenSysId = true
				foundSupertokenSysIdOnIdx = idx

				if identifier.Value == superTokenUserID {
					supertokenSysIdExactMatch = true
				}
			}

			if identifier.System == constvars.KonsulinOmnichannelSystemIdentifier {
				foundChatwootID = true
				foundChatwootIDOnIdx = idx

				if identifier.Value == chatwootID {
					chatwootIDExactMatch = true
				}
			}
		}

		if superTokenUserID != "" {
			if foundSupertokenSysId && !supertokenSysIdExactMatch {
				mustUpdatePatient = true
				patient.Identifier[foundSupertokenSysIdOnIdx] = fhir_dto.Identifier{
					System: constvars.FhirSupertokenSystemIdentifier,
					Value:  superTokenUserID,
				}
			}

			if !foundSupertokenSysId {
				mustUpdatePatient = true
				patient.Identifier = append(patient.Identifier, fhir_dto.Identifier{
					System: constvars.FhirSupertokenSystemIdentifier,
					Value:  superTokenUserID,
				})
			}
		}

		// only attempt to update the patient identifier with the chatwoot ID if the chatwoot call was successful
		// and the chatwoot ID is not 0 (which can means that the user is not yet added to the chatwoot workspace or the API call failed)
		if chatwootCallErr == nil && userChatwootContact.ChatwootID != 0 {
			if !foundChatwootID {
				mustUpdatePatient = true
				patient.Identifier = append(patient.Identifier, fhir_dto.Identifier{
					System: constvars.KonsulinOmnichannelSystemIdentifier,
					Value:  chatwootID,
				})
			}

			if foundChatwootID && !chatwootIDExactMatch {
				mustUpdatePatient = true
				patient.Identifier[foundChatwootIDOnIdx] = fhir_dto.Identifier{
					System: constvars.KonsulinOmnichannelSystemIdentifier,
					Value:  chatwootID,
				}
			}
		}

		if mustUpdatePatient {
			updatedPatient, err := uc.PatientFhirClient.UpdatePatient(ctx, &patient)
			if err != nil {
				return nil, err
			}

			return updatedPatient, nil
		}

		return &patient, nil
	}

	userChatwootContact, chatwootErr := uc.callWebhookSvcKonsulinOmnichannel(ctx, email, "")
	if chatwootErr != nil {
		// log the error but continue the process
		uc.Log.Error("userUsecase.createPatientIfNotExists error calling webhook svc konsulin omnichannel",
			zap.Error(chatwootErr),
		)
	}
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

	newPatientInput := &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		Active:       true,
		Identifier:   []fhir_dto.Identifier{},
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemEmail,
				Value:  email,
				Use:    "work",
			},
		},
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

	newPatient, err := uc.PatientFhirClient.CreatePatient(ctx, newPatientInput)
	if err != nil {
		return nil, err
	}

	return newPatient, nil
}

func (uc *userUsecase) createPersonIfNotExists(ctx context.Context, email string, superTokenUserID string) (*fhir_dto.Person, error) {
	person, err := uc.PersonFhirClient.FindPersonByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if len(person) > 0 {
		person := person[0]

		found := false
		foundOnIdx := -1
		exactMatch := false

		for idx, identifier := range person.Identifier {
			if identifier.System == constvars.FhirSupertokenSystemIdentifier {
				found = true
				foundOnIdx = idx

				if identifier.Value == superTokenUserID {
					exactMatch = true
					break
				}
			}
		}

		if found && !exactMatch {
			if superTokenUserID == "" {
				return &person, nil
			}

			person.Identifier[foundOnIdx] = fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			}

			updatedPerson, err := uc.PersonFhirClient.Update(ctx, &person)
			if err != nil {
				return nil, err
			}

			return updatedPerson, nil
		}

		if !found {
			if superTokenUserID == "" {
				return &person, nil
			}

			person.Identifier = append(person.Identifier, fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			})

			updatedPerson, err := uc.PersonFhirClient.Update(ctx, &person)
			if err != nil {
				return nil, err
			}

			return updatedPerson, nil
		}

		return &person, nil
	}

	newPersonInput := &fhir_dto.Person{
		ResourceType: constvars.ResourcePerson,
		Active:       true,
		Identifier:   []fhir_dto.Identifier{},
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemEmail,
				Value:  email,
				Use:    "work",
			},
		},
	}

	if superTokenUserID != "" {
		newPersonInput.Identifier = append(newPersonInput.Identifier, fhir_dto.Identifier{
			System: constvars.FhirSupertokenSystemIdentifier,
			Value:  superTokenUserID,
		})
	}

	newPerson, err := uc.PersonFhirClient.Create(ctx, newPersonInput)
	if err != nil {
		return nil, err
	}

	return newPerson, nil
}

func (uc *userUsecase) updatePatientFhirProfile(ctx context.Context, user *models.User, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.updatePatientFhirProfile called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, session.PatientID),
	)

	sessionModel := models.Session{
		UserID:    user.ID,
		PatientID: user.PatientID,
		Email:     user.Email,
		Username:  user.Username,
		RoleID:    session.RoleID,
		RoleName:  session.RoleName,
		SessionID: session.SessionID,
	}

	err := uc.RedisRepository.Set(ctx, session.SessionID, sessionModel, time.Hour)
	if err != nil {
		uc.Log.Error("userUsecase.updatePatientFhirProfile error storing updated session in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	patientFhirRequest := utils.BuildFhirPatientUpdateProfileRequest(request, session.PatientID)
	fhirPatient, err := uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
	if err != nil {
		uc.Log.Error("userUsecase.updatePatientFhirProfile error updating patient FHIR resource",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("userUsecase.updatePatientFhirProfile succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, fhirPatient.ID),
	)

	response := &responses.UpdateUserProfile{
		PatientID: fhirPatient.ID,
	}
	return response, nil
}

func (uc *userUsecase) updatePractitionerFhirProfile(ctx context.Context, user *models.User, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.updatePractitionerFhirProfile called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, session.PractitionerID),
	)

	sessionModel := models.Session{
		UserID:         user.ID,
		PractitionerID: user.PractitionerID,
		Email:          user.Email,
		Username:       user.Username,
		RoleID:         session.RoleID,
		RoleName:       session.RoleName,
		SessionID:      session.SessionID,
	}

	err := uc.RedisRepository.Set(ctx, session.SessionID, sessionModel, time.Hour)
	if err != nil {
		uc.Log.Error("userUsecase.updatePractitionerFhirProfile error storing updated session in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	practitionerFhirRequest := utils.BuildFhirPractitionerUpdateProfileRequest(request, session.PractitionerID)
	fhirPractitioner, err := uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitionerFhirRequest)
	if err != nil {
		uc.Log.Error("userUsecase.updatePractitionerFhirProfile error updating practitioner FHIR resource",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("userUsecase.updatePractitionerFhirProfile succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, fhirPractitioner.ID),
	)

	response := &responses.UpdateUserProfile{
		PractitionerID: fhirPractitioner.ID,
	}
	return response, nil
}

func (uc *userUsecase) getPatientProfile(ctx context.Context, session *models.Session, preSignedUrl string) (*responses.UserProfile, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.getPatientProfile called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, session.PatientID),
	)

	patientFhir, err := uc.PatientFhirClient.FindPatientByID(ctx, session.PatientID)
	if err != nil {
		uc.Log.Error("userUsecase.getPatientProfile error fetching patient FHIR resource",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response := utils.BuildPatientProfileResponse(patientFhir)
	response.ProfilePicture = preSignedUrl

	uc.Log.Info("userUsecase.getPatientProfile succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}

func (uc *userUsecase) getPractitionerProfile(ctx context.Context, session *models.Session, preSignedUrl string) (*responses.UserProfile, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("userUsecase.getPractitionerProfile called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, session.PractitionerID),
	)

	practitionerFhir, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, session.PractitionerID)
	if err != nil {
		uc.Log.Error("userUsecase.getPractitionerProfile error fetching practitioner FHIR resource",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response := utils.BuildPractitionerProfileResponse(practitionerFhir)
	response.ProfilePicture = preSignedUrl

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, session.PractitionerID)
	if err != nil {
		uc.Log.Error("userUsecase.getPractitionerProfile error fetching practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	practiceInformations := make([]responses.PracticeInformation, 0, len(practitionerRoles))
	practiceAvailabilities := make([]responses.PracticeAvailability, 0, len(practitionerRoles))
	for _, practitionerRole := range practitionerRoles {
		organizationID := strings.Split(practitionerRole.Organization.Reference, "/")[1]
		organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, organizationID)
		if err != nil {
			uc.Log.Error("userUsecase.getPractitionerProfile error fetching organization",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingOrganizationIDKey, organizationID),
				zap.Error(err),
			)
			return nil, err
		}
		practiceInformations = append(practiceInformations, responses.PracticeInformation{
			ClinicID:    organization.ID,
			ClinicName:  organization.Name,
			Affiliation: organization.Name,
			Specialties: utils.ExtractSpecialties(practitionerRole.Specialty),
			PricePerSession: responses.PricePerSession{
				Value:    practitionerRole.Extension[0].ValueMoney.Value,
				Currency: practitionerRole.Extension[0].ValueMoney.Currency,
			},
		})
		if len(practitionerRole.AvailableTime) > 0 {
			practiceAvailabilities = append(practiceAvailabilities, responses.PracticeAvailability{
				ClinicID:       organization.ID,
				AvailableTimes: utils.ConvertToAvailableTimesResponse(practitionerRole.AvailableTime),
			})
		}
	}
	response.PracticeInformations = practiceInformations
	response.PracticeAvailabilities = practiceAvailabilities

	uc.Log.Info("userUsecase.getPractitionerProfile succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}



type callWebhookSvcKonsulinOmnichannelOutput struct {
	ChatwootID int    `json:"chatwoot_id"`
	Email      string `json:"email"`
}

func (uc *userUsecase) callWebhookSvcKonsulinOmnichannel(ctx context.Context, email, username string) (callWebhookSvcKonsulinOmnichannelOutput, error) {
	lastUsername := username
	if lastUsername == "" {
		lastUsername = strings.Split(email, "@")[0]
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

	url := fmt.Sprintf("%s/synchronous/modify-profile", uc.InternalConfig.Webhook.URL)

	body := struct {
		Email    string `json:"email"`
		Username string `json:"username"`
	}{
		Email:    email,
		Username: lastUsername,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}

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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return callWebhookSvcKonsulinOmnichannelOutput{}, errors.New("failed to call webhook svc konsulin omnichannel")
	}

	bodyBytesResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}

	var outputs []callWebhookSvcKonsulinOmnichannelOutput
	if err = json.Unmarshal(bodyBytesResp, &outputs); err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}
	if len(outputs) == 0 {
		return callWebhookSvcKonsulinOmnichannelOutput{}, errors.New("webhook svc konsulin omnichannel returned empty response")
	}

	output := outputs[0]
	return output, nil
}
