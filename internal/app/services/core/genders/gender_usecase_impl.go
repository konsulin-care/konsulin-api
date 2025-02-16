package genders

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

type genderUsecase struct {
	GenderRepository contracts.GenderRepository
	RedisRepository  contracts.RedisRepository
	Log              *zap.Logger
}

var (
	genderUsecaseInstance contracts.GenderUsecase
	onceGenderUsecase     sync.Once
	genderUsecaseError    error
)

func NewGenderUsecase(
	genderPostgresRepository contracts.GenderRepository,
	redisRepository contracts.RedisRepository,
	logger *zap.Logger,
) (contracts.GenderUsecase, error) {
	onceGenderUsecase.Do(func() {
		instance := &genderUsecase{
			GenderRepository: genderPostgresRepository,
			RedisRepository:  redisRepository,
			Log:              logger,
		}

		ctx := context.Background()
		err := instance.initializeData(ctx)
		if err != nil {
			genderUsecaseError = err
			return
		}
		genderUsecaseInstance = instance
	})

	return genderUsecaseInstance, genderUsecaseError
}
func (uc *genderUsecase) FindAll(ctx context.Context) ([]responses.Gender, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("genderUsecase.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	var genders []models.Gender

	genderRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyGenderList)
	if err != nil {
		uc.Log.Error("genderUsecase.FindAll error retrieving data from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if genderRedisData == "" {
		uc.Log.Info("genderUsecase.FindAll no data found in Redis, fetching from MongoDB",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		genders, err = uc.GenderRepository.FindAll(ctx)
		if err != nil {
			uc.Log.Error("genderUsecase.FindAll error fetching data from MongoDB",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyGenderList, genders, 0)
		if err != nil {
			uc.Log.Error("genderUsecase.FindAll error caching data in Redis",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		uc.Log.Info("genderUsecase.FindAll successfully fetched and cached data from MongoDB",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	} else {
		uc.Log.Info("genderUsecase.FindAll data found in Redis, parsing JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		err = json.Unmarshal([]byte(genderRedisData), &genders)
		if err != nil {
			uc.Log.Error("genderUsecase.FindAll error parsing JSON from Redis",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCannotParseJSON(err)
		}
	}

	response := make([]responses.Gender, len(genders))
	for i, eachGender := range genders {
		response[i] = eachGender.ConvertIntoResponse()
	}

	uc.Log.Info("genderUsecase.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int("gender_count", len(response)),
	)
	return response, nil
}

func (uc *genderUsecase) initializeData(ctx context.Context) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("genderUsecase.initializeData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	genderRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyGenderList)
	if err != nil {
		uc.Log.Error("genderUsecase.initializeData error retrieving data from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if genderRedisData == "" {
		uc.Log.Info("genderUsecase.initializeData no data found in Redis, fetching from MongoDB",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		genders, err := uc.GenderRepository.FindAll(ctx)
		if err != nil {
			uc.Log.Error("genderUsecase.initializeData error fetching data from MongoDB",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}

		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyGenderList, genders, 0)
		if err != nil {
			uc.Log.Error("genderUsecase.initializeData error caching data in Redis",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}
		uc.Log.Info("genderUsecase.initializeData successfully fetched and cached data from MongoDB",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	} else {
		uc.Log.Info("genderUsecase.initializeData data already exists in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	}

	uc.Log.Info("genderUsecase.initializeData completed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}
