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
	var cities []models.City

	// Retrieve the 'city' data from Redis
	cityRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyCityList)
	if err != nil {
		return nil, err
	}

	if cityRedisData == "" {
		// Fetch data from MongoDB if not found in Redis
		cities, err = uc.CityRepository.FindAll(ctx)
		if err != nil {
			return nil, err
		}

		// Cache the data in Redis
		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyCityList, cities, 0)
		if err != nil {
			return nil, err
		}
	} else {
		// Parse the data from Redis
		err = json.Unmarshal([]byte(cityRedisData), &cities)
		if err != nil {
			return nil, exceptions.ErrCannotParseJSON(err)
		}
	}

	filteredCities := uc.filterCitiesBySearch(cities, queryParamsRequest)

	// Build the response
	response := make([]responses.City, len(filteredCities))
	for i, eachCity := range filteredCities {
		response[i] = eachCity.ConvertIntoResponse()
	}

	return response, nil
}

func (uc *cityUsecase) initializeData(ctx context.Context) error {
	cityRedisData, err := uc.RedisRepository.Get(ctx, constvars.RedisKeyCityList)
	if err != nil {
		return err
	}

	// If 'cityRedisData' is empty
	if cityRedisData == "" {
		// Retrieve data from MongoDB
		cities, err := uc.CityRepository.FindAll(ctx)
		if err != nil {
			return err
		}

		// Cache the data in Redis
		err = uc.RedisRepository.Set(ctx, constvars.RedisKeyCityList, cities, 0)
		if err != nil {
			return err
		}
	}

	return nil
}

func (uc *cityUsecase) filterCitiesBySearch(cities []models.City, queryParamsRequest *requests.QueryParams) []models.City {
	if queryParamsRequest.Search == "" {
		return cities
	}

	queryParamsRequest.Search = strings.ToLower(queryParamsRequest.Search)
	filteredCities := make([]models.City, 0)

	for _, city := range cities {
		if strings.Contains(strings.ToLower(city.Name), queryParamsRequest.Search) {
			filteredCities = append(filteredCities, city)
		}
	}

	return filteredCities
}
