package educationLevels

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"sync"

	"go.uber.org/zap"
)

type educationLevelUsecase struct {
	EducationLevelRepository contracts.EducationLevelRepository
	RedisRepository          contracts.RedisRepository
	Log                      *zap.Logger
}

var (
	educationLevelUsecaseInstance contracts.EducationLevelUsecase
	onceEducationLevelUsecase     sync.Once
	educationLevelUsecaseError    error
)

func NewEducationLevelUsecase(
	educationLevelPostgresRepository contracts.EducationLevelRepository,
	redisRepository contracts.RedisRepository,
	logger *zap.Logger,
) (contracts.EducationLevelUsecase, error) {
	onceEducationLevelUsecase.Do(func() {
		instance := &educationLevelUsecase{
			EducationLevelRepository: educationLevelPostgresRepository,
			RedisRepository:          redisRepository,
			Log:                      logger,
		}

		ctx := context.Background()
		err := instance.initializeData(ctx)
		if err != nil {
			educationLevelUsecaseError = err
			return
		}
		educationLevelUsecaseInstance = instance
	})

	return educationLevelUsecaseInstance, educationLevelUsecaseError
}
func (uc *educationLevelUsecase) FindAll(ctx context.Context) ([]responses.EducationLevel, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("educationLevelUsecase.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	var educationLevels []models.EducationLevel

	educationLevelRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyEducationLevelList)
	if err != nil {
		uc.Log.Error("educationLevelUsecase.FindAll error retrieving data from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if educationLevelRedisData == "" {
		uc.Log.Info("educationLevelUsecase.FindAll no data in Redis; fetching from MongoDB",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		educationLevels, err = uc.EducationLevelRepository.FindAll(ctx)
		if err != nil {
			uc.Log.Error("educationLevelUsecase.FindAll error fetching data from MongoDB",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyEducationLevelList, educationLevels, 0)
		if err != nil {
			uc.Log.Error("educationLevelUsecase.FindAll error caching data in Redis",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		uc.Log.Info("educationLevelUsecase.FindAll successfully fetched and cached data from MongoDB",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	} else {
		uc.Log.Info("educationLevelUsecase.FindAll data found in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		err = json.Unmarshal([]byte(educationLevelRedisData), &educationLevels)
		if err != nil {
			uc.Log.Error("educationLevelUsecase.FindAll error parsing JSON from Redis",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCannotParseJSON(err)
		}
	}

	response := make([]responses.EducationLevel, len(educationLevels))
	for i, eachEducationLevel := range educationLevels {
		response[i] = eachEducationLevel.ConvertIntoResponse()
	}

	uc.Log.Info("educationLevelUsecase.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingEducationLevelCountKey, len(response)),
	)
	return response, nil
}

func (uc *educationLevelUsecase) initializeData(ctx context.Context) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("educationLevelUsecase.initializeData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	educationLevelRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyEducationLevelList)
	if err != nil {
		uc.Log.Error("educationLevelUsecase.initializeData error retrieving data from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if educationLevelRedisData == "" {
		uc.Log.Info("educationLevelUsecase.initializeData no data in Redis; fetching from MongoDB",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		educationLevels, err := uc.EducationLevelRepository.FindAll(ctx)
		if err != nil {
			uc.Log.Error("educationLevelUsecase.initializeData error fetching data from MongoDB",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}

		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyEducationLevelList, educationLevels, 0)
		if err != nil {
			uc.Log.Error("educationLevelUsecase.initializeData error caching data in Redis",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}
		uc.Log.Info("educationLevelUsecase.initializeData successfully fetched and cached data from MongoDB",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	} else {
		uc.Log.Info("educationLevelUsecase.initializeData data already exists in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	}

	uc.Log.Info("educationLevelUsecase.initializeData completed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}
