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
	var genders []models.Gender

	// Retrieve the 'gender' data from Redis
	genderRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyGenderList)
	if err != nil {
		return nil, err
	}

	if genderRedisData == "" {
		// Fetch data from MongoDB if not found in Redis
		genders, err = uc.GenderRepository.FindAll(ctx)
		if err != nil {
			return nil, err
		}

		// Cache the data in Redis
		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyGenderList, genders, 0)
		if err != nil {
			return nil, err
		}
	} else {
		// Parse the data from Redis
		err = json.Unmarshal([]byte(genderRedisData), &genders)
		if err != nil {
			return nil, exceptions.ErrCannotParseJSON(err)
		}
	}

	// Build the response
	response := make([]responses.Gender, len(genders))
	for i, eachGender := range genders {
		response[i] = eachGender.ConvertIntoResponse()
	}

	return response, nil
}

func (uc *genderUsecase) initializeData(ctx context.Context) error {
	genderRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyGenderList)
	if err != nil {
		return err
	}

	// If 'genderRedisData' is empty
	if genderRedisData == "" {
		// Retrieve data from MongoDB
		genders, err := uc.GenderRepository.FindAll(ctx)
		if err != nil {
			return err
		}

		// Cache the data in Redis
		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyGenderList, genders, 0)
		if err != nil {
			return err
		}
	}

	return nil
}
