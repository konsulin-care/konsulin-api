package educationLevels

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
)

type educationLevelUsecase struct {
	EducationLevelRepository EducationLevelRepository
	RedisRepository          redis.RedisRepository
}

func NewEducationLevelUsecase(
	educationLevelMongoRepository EducationLevelRepository,
	redisRepository redis.RedisRepository,
) (EducationLevelUsecase, error) {
	educationLevelUsecase := &educationLevelUsecase{
		EducationLevelRepository: educationLevelMongoRepository,
		RedisRepository:          redisRepository,
	}

	ctx := context.Background()
	err := educationLevelUsecase.initializeData(ctx)
	if err != nil {
		return nil, err
	}

	return educationLevelUsecase, nil
}

func (uc *educationLevelUsecase) FindAll(ctx context.Context) ([]responses.EducationLevel, error) {
	var educationLevels []models.EducationLevel

	// Retrieve the 'educationLevel' data from Redis
	educationLevelRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyEducationLevelList)
	if err != nil {
		return nil, err
	}

	if educationLevelRedisData == "" {
		// Fetch data from MongoDB if not found in Redis
		educationLevels, err = uc.EducationLevelRepository.FindAll(ctx)
		if err != nil {
			return nil, err
		}

		// Cache the data in Redis
		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyEducationLevelList, educationLevels, 0)
		if err != nil {
			return nil, err
		}
	} else {
		// Parse the data from Redis
		err = json.Unmarshal([]byte(educationLevelRedisData), &educationLevels)
		if err != nil {
			return nil, exceptions.ErrCannotParseJSON(err)
		}
	}

	// Build the response
	response := make([]responses.EducationLevel, len(educationLevels))
	for i, eachEducationLevel := range educationLevels {
		response[i] = eachEducationLevel.ConvertIntoResponse()
	}

	return response, nil
}

func (uc *educationLevelUsecase) initializeData(ctx context.Context) error {
	educationLevelRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyEducationLevelList)
	if err != nil {
		return err
	}

	// If 'educationLevelRedisData' is empty
	if educationLevelRedisData == "" {
		// Retrieve data from MongoDB
		educationLevels, err := uc.EducationLevelRepository.FindAll(ctx)
		if err != nil {
			return err
		}

		// Cache the data in Redis
		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyEducationLevelList, educationLevels, 0)
		if err != nil {
			return err
		}
	}

	return nil
}
