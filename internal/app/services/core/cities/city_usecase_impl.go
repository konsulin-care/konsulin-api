package cities

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"strings"
	"sync"

	"go.uber.org/zap"
)

type cityUsecase struct {
	CityRepository  contracts.CityRepository
	RedisRepository contracts.RedisRepository
	Log             *zap.Logger
}

var (
	cityUsecaseInstance contracts.CityUsecase
	onceCityUsecase     sync.Once
	cityUsecaseError    error
)

func NewCityUsecase(
	cityPostgresRepository contracts.CityRepository,
	redisRepository contracts.RedisRepository,
	logger *zap.Logger,
) (contracts.CityUsecase, error) {
	onceCityUsecase.Do(func() {
		instance := &cityUsecase{
			CityRepository:  cityPostgresRepository,
			RedisRepository: redisRepository,
			Log:             logger,
		}

		ctx := context.Background()
		err := instance.initializeData(ctx)
		if err != nil {
			cityUsecaseError = err
			return
		}
		cityUsecaseInstance = instance
	})

	return cityUsecaseInstance, cityUsecaseError
}

func (uc *cityUsecase) FindAll(ctx context.Context, queryParamsRequest *requests.QueryParams) ([]responses.City, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("cityUsecase.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	var cities []models.City

	cityRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyCityList)
	if err != nil {
		uc.Log.Error("cityUsecase.FindAll error retrieving city data from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if cityRedisData == "" {
		uc.Log.Info("cityUsecase.FindAll no data in Redis, fetching from repository",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		cities, err = uc.CityRepository.FindAll(ctx)
		if err != nil {
			uc.Log.Error("cityUsecase.FindAll error fetching cities from repository",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		uc.Log.Info("cityUsecase.FindAll fetched cities from repository",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int(constvars.LoggingCitiesCountKey, len(cities)),
		)

		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyCityList, cities, 0)
		if err != nil {
			uc.Log.Error("cityUsecase.FindAll error caching cities in Redis",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
	} else {
		uc.Log.Info("cityUsecase.FindAll data found in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		err = json.Unmarshal([]byte(cityRedisData), &cities)
		if err != nil {
			uc.Log.Error("cityUsecase.FindAll error unmarshaling Redis data",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCannotParseJSON(err)
		}
	}

	filteredCities := uc.filterCitiesBySearch(ctx, cities, queryParamsRequest)
	uc.Log.Info("cityUsecase.FindAll filtered cities",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingCitiesCountKey, len(filteredCities)),
	)

	response := make([]responses.City, len(filteredCities))
	for i, eachCity := range filteredCities {
		response[i] = eachCity.ConvertIntoResponse()
	}

	uc.Log.Info("cityUsecase.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(response)),
	)
	return response, nil
}

func (uc *cityUsecase) initializeData(ctx context.Context) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("cityUsecase.initializeData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	cityRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyCityList)
	if err != nil {
		uc.Log.Error("cityUsecase.initializeData error retrieving city data from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if cityRedisData == "" {
		uc.Log.Info("cityUsecase.initializeData no data in Redis, fetching from repository",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		cities, err := uc.CityRepository.FindAll(ctx)
		if err != nil {
			uc.Log.Error("cityUsecase.initializeData error fetching cities from repository",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}

		uc.Log.Info("cityUsecase.initializeData fetched cities",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int(constvars.LoggingCitiesCountKey, len(cities)),
		)

		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyCityList, cities, 0)
		if err != nil {
			uc.Log.Error("cityUsecase.initializeData error caching cities in Redis",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}
		uc.Log.Info("cityUsecase.initializeData cached cities in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	} else {
		uc.Log.Info("cityUsecase.initializeData found data in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	}
	return nil
}

func (uc *cityUsecase) filterCitiesBySearch(ctx context.Context, cities []models.City, queryParamsRequest *requests.QueryParams) []models.City {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("cityUsecase.filterCitiesBySearch called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	if queryParamsRequest.Search == "" {
		uc.Log.Info("cityUsecase.filterCitiesBySearch no search term provided",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return cities
	}

	searchTerm := strings.ToLower(queryParamsRequest.Search)
	uc.Log.Info("cityUsecase.filterCitiesBySearch filtering cities",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSearchTermKey, searchTerm),
	)

	filteredCities := make([]models.City, 0)
	for _, city := range cities {
		if strings.Contains(strings.ToLower(city.Name), searchTerm) {
			filteredCities = append(filteredCities, city)
		}
	}
	uc.Log.Info("cityUsecase.filterCitiesBySearch completed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingCitiesCountKey, len(filteredCities)),
	)
	return filteredCities
}
