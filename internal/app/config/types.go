package config

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type (
	Bootstrap struct {
		App            *fiber.App
		MongoDB        *mongo.Database
		Redis          *redis.Client
		DriverConfig   *DriverConfig
		InternalConfig *InternalConfig
	}

	InternalConfig struct {
		FHIR FHIR
		JWT  JWT
	}

	DriverConfig struct {
		App     App
		MongoDB MongoDB
		Redis   Redis
	}

	MongoDB struct {
		Port     string
		Host     string
		DbName   string
		Username string
		Password string
	}

	App struct {
		Env                        string
		Port                       string
		Version                    string
		Timezone                   string
		EndpointPrefix             string
		MaxRequests                int
		ShutdownTimeout            int
		MaxTimeRequestsInSeconds   int
		RequestBodyLimitInMegabyte int
	}

	FHIR struct {
		BaseUrl string
	}

	Redis struct {
		Port     string
		Password string
	}

	JWT struct {
		Secret string
	}
)
