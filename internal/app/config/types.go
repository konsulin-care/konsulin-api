package config

import (
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type (
	Bootstrap struct {
		Router         *chi.Mux
		MongoDB        *mongo.Database
		Redis          *redis.Client
		Logger         *zap.Logger
		DriverConfig   *DriverConfig
		InternalConfig *InternalConfig
	}

	InternalConfig struct {
		App  App
		FHIR FHIR
		JWT  JWT
	}

	DriverConfig struct {
		MongoDB MongoDB
		Redis   Redis
		Logger  Logger
	}

	App struct {
		Env                        string
		Port                       string
		Version                    string
		Address                    string
		Timezone                   string
		EndpointPrefix             string
		MaxRequests                int
		ShutdownTimeout            int
		MaxTimeRequestsPerSeconds  int
		RequestBodyLimitInMegabyte int
	}

	MongoDB struct {
		Port     string
		Host     string
		DbName   string
		Username string
		Password string
	}
	Redis struct {
		Host     string
		Port     string
		Password string
	}
	Logger struct {
		Level               string
		OutputFileName      string
		OutputErrorFileName string
	}
	FHIR struct {
		BaseUrl string
	}

	JWT struct {
		Secret string
	}
)
