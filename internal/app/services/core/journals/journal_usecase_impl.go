package journals

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
)

type journalUsecase struct {
	SessionService        contracts.SessionService
	ObservationFhirClient contracts.ObservationFhirClient
	RedisRepository       contracts.RedisRepository
	InternalConfig        *config.InternalConfig
}

func NewJournalUsecase(
	sessionService contracts.SessionService,
	observationFhirClient contracts.ObservationFhirClient,
	redisRepository contracts.RedisRepository,
	internalConfig *config.InternalConfig,
) contracts.JournalUsecase {
	return &journalUsecase{
		SessionService:        sessionService,
		ObservationFhirClient: observationFhirClient,
		RedisRepository:       redisRepository,
		InternalConfig:        internalConfig,
	}
}

func (uc *journalUsecase) CreateJournal(ctx context.Context, request *requests.CreateJournal) (*responses.Journal, error) {
	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		return nil, err
	}

	if session.IsNotPatient() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	fhirObservation, err := utils.MapJournalRequestToCreateObserVationRequest(request)
	if err != nil {
		return nil, err
	}

	observation, err := uc.ObservationFhirClient.CreateObservation(ctx, fhirObservation)
	if err != nil {
		return nil, err
	}

	response, err := utils.MapObservationToJournalResponse(observation)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (uc *journalUsecase) UpdateJournal(ctx context.Context, request *requests.UpdateJournal) (*responses.Journal, error) {
	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		return nil, err
	}

	if session.IsNotPatient() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	isOwner, err := uc.isUserTheOwnerOfThisJournal(ctx, request.JournalID, session)
	if err != nil || !isOwner {
		return nil, err
	}

	fhirObservation, err := utils.MapUpdateJournalToUpdateObservationRequest(request)
	if err != nil {
		return nil, err
	}

	observation, err := uc.ObservationFhirClient.UpdateObservation(ctx, fhirObservation)
	if err != nil {
		return nil, err
	}

	response, err := utils.MapObservationToJournalResponse(observation)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (uc *journalUsecase) FindJournalByID(ctx context.Context, request *requests.FindJournalByID) (*responses.Journal, error) {
	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		return nil, err
	}

	if session.IsNotPatient() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	observation, err := uc.ObservationFhirClient.FindObservationByID(ctx, request.JournalID)
	if err != nil {
		return nil, err
	}

	response, err := utils.MapObservationToJournalResponse(observation)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (uc *journalUsecase) DeleteJournalByID(ctx context.Context, request *requests.DeleteJournalByID) error {
	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		return err
	}

	if session.IsNotPatient() {
		return exceptions.ErrNotMatchRoleType(nil)
	}

	isOwner, err := uc.isUserTheOwnerOfThisJournal(ctx, request.JournalID, session)
	if err != nil || !isOwner {
		return err
	}

	err = uc.ObservationFhirClient.DeleteObservationByID(ctx, request.JournalID)
	if err != nil {
		return err
	}

	return nil
}

func (uc *journalUsecase) isUserTheOwnerOfThisJournal(ctx context.Context, journalID string, session *models.Session) (bool, error) {
	observation, err := uc.ObservationFhirClient.FindObservationByID(ctx, journalID)
	if err != nil {
		return false, err
	}

	patientID, err := utils.GetPatientIDFromObservation(observation)
	if err != nil {
		return false, err
	}

	if patientID != session.PatientID {
		return false, exceptions.ErrAuthInvalidRole(nil)
	}

	return true, nil
}
