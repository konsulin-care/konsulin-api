package journals

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"sync"

	"go.uber.org/zap"
)

type journalUsecase struct {
	SessionService        contracts.SessionService
	ObservationFhirClient contracts.ObservationFhirClient
	RedisRepository       contracts.RedisRepository
	InternalConfig        *config.InternalConfig
	Log                   *zap.Logger
}

var (
	journalUsecaseInstance contracts.JournalUsecase
	onceJournalUsecase     sync.Once
)

func NewJournalUsecase(
	sessionService contracts.SessionService,
	observationFhirClient contracts.ObservationFhirClient,
	redisRepository contracts.RedisRepository,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
) contracts.JournalUsecase {
	onceJournalUsecase.Do(func() {
		instance := &journalUsecase{
			SessionService:        sessionService,
			ObservationFhirClient: observationFhirClient,
			RedisRepository:       redisRepository,
			InternalConfig:        internalConfig,
			Log:                   logger,
		}
		journalUsecaseInstance = instance
	})
	return journalUsecaseInstance
}
func (uc *journalUsecase) CreateJournal(ctx context.Context, request *requests.CreateJournal) (*responses.Journal, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("journalUsecase.CreateJournal called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		uc.Log.Error("journalUsecase.CreateJournal error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if session.IsNotPatient() {
		uc.Log.Error("journalUsecase.CreateJournal error: session role is not patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	request.PatientID = session.PatientID
	fhirObservation, err := utils.MapJournalRequestToCreateObserVationRequest(request)
	if err != nil {
		uc.Log.Error("journalUsecase.CreateJournal error mapping request to FHIR observation",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	observation, err := uc.ObservationFhirClient.CreateObservation(ctx, fhirObservation)
	if err != nil {
		uc.Log.Error("journalUsecase.CreateJournal error creating observation in FHIR client",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response, err := utils.MapObservationToJournalResponse(observation)
	if err != nil {
		uc.Log.Error("journalUsecase.CreateJournal error mapping observation to journal response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("journalUsecase.CreateJournal succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, observation.ID),
	)
	return response, nil
}

func (uc *journalUsecase) UpdateJournal(ctx context.Context, request *requests.UpdateJournal) (*responses.Journal, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("journalUsecase.UpdateJournal called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		uc.Log.Error("journalUsecase.UpdateJournal error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if session.IsNotPatient() {
		uc.Log.Error("journalUsecase.UpdateJournal error: session role is not patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	request.PatientID = session.PatientID
	isOwner, err := uc.isUserTheOwnerOfThisJournal(ctx, request.JournalID, session)
	if err != nil || !isOwner {
		uc.Log.Error("journalUsecase.UpdateJournal error: user is not owner of journal",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingJournalIDKey, request.JournalID),
			zap.Error(err),
		)
		return nil, err
	}

	fhirObservation, err := utils.MapUpdateJournalToUpdateObservationRequest(request)
	if err != nil {
		uc.Log.Error("journalUsecase.UpdateJournal error mapping update request to FHIR observation",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	observation, err := uc.ObservationFhirClient.UpdateObservation(ctx, fhirObservation)
	if err != nil {
		uc.Log.Error("journalUsecase.UpdateJournal error updating observation in FHIR client",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response, err := utils.MapObservationToJournalResponse(observation)
	if err != nil {
		uc.Log.Error("journalUsecase.UpdateJournal error mapping updated observation to journal response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("journalUsecase.UpdateJournal succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, observation.ID),
	)
	return response, nil
}

func (uc *journalUsecase) FindJournalByID(ctx context.Context, request *requests.FindJournalByID) (*responses.Journal, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("journalUsecase.FindJournalByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, request.JournalID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		uc.Log.Error("journalUsecase.FindJournalByID error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if session.IsNotPatient() {
		uc.Log.Error("journalUsecase.FindJournalByID error: session role is not patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	observation, err := uc.ObservationFhirClient.FindObservationByID(ctx, request.JournalID)
	if err != nil {
		uc.Log.Error("journalUsecase.FindJournalByID error fetching observation from FHIR client",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingJournalIDKey, request.JournalID),
			zap.Error(err),
		)
		return nil, err
	}

	response, err := utils.MapObservationToJournalResponse(observation)
	if err != nil {
		uc.Log.Error("journalUsecase.FindJournalByID error mapping observation to journal response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("journalUsecase.FindJournalByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, request.JournalID),
	)
	return response, nil
}

func (uc *journalUsecase) DeleteJournalByID(ctx context.Context, request *requests.DeleteJournalByID) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("journalUsecase.DeleteJournalByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, request.JournalID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		uc.Log.Error("journalUsecase.DeleteJournalByID error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if session.IsNotPatient() {
		uc.Log.Error("journalUsecase.DeleteJournalByID error: session role is not patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.ErrNotMatchRoleType(nil)
	}

	isOwner, err := uc.isUserTheOwnerOfThisJournal(ctx, request.JournalID, session)
	if err != nil || !isOwner {
		uc.Log.Error("journalUsecase.DeleteJournalByID error: user is not owner of journal",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingJournalIDKey, request.JournalID),
			zap.Error(err),
		)
		return err
	}

	err = uc.ObservationFhirClient.DeleteObservationByID(ctx, request.JournalID)
	if err != nil {
		uc.Log.Error("journalUsecase.DeleteJournalByID error deleting observation from FHIR client",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingJournalIDKey, request.JournalID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("journalUsecase.DeleteJournalByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, request.JournalID),
	)
	return nil
}

func (uc *journalUsecase) isUserTheOwnerOfThisJournal(ctx context.Context, journalID string, session *models.Session) (bool, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("journalUsecase.isUserTheOwnerOfThisJournal called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, journalID),
	)

	observation, err := uc.ObservationFhirClient.FindObservationByID(ctx, journalID)
	if err != nil {
		uc.Log.Error("journalUsecase.isUserTheOwnerOfThisJournal error fetching observation",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingJournalIDKey, journalID),
			zap.Error(err),
		)
		return false, err
	}

	patientID, err := utils.GetPatientIDFromObservation(observation)
	if err != nil {
		uc.Log.Error("journalUsecase.isUserTheOwnerOfThisJournal error retrieving patient ID from observation",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return false, err
	}

	if patientID != session.PatientID {
		uc.Log.Error("journalUsecase.isUserTheOwnerOfThisJournal error: patient ID mismatch",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("expected_patient_id", session.PatientID),
			zap.String("found_patient_id", patientID),
		)
		return false, exceptions.ErrAuthInvalidRole(nil)
	}

	uc.Log.Info("journalUsecase.isUserTheOwnerOfThisJournal succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return true, nil
}
