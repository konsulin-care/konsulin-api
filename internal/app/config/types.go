package config

import (
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

type (
	Bootstrap struct {
		Router         *chi.Mux
		MongoDB        *mongo.Database
		Redis          *redis.Client
		Logger         *logrus.Logger
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
	}

	App struct {
		Env                        string
		Port                       string
		Version                    string
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
		Port     string
		Password string
	}
	FHIR struct {
		BaseUrl string
	}

	JWT struct {
		Secret string
	}
)
