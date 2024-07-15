package main

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/database"
	"konsulin-service/internal/app/drivers/logger"
	"konsulin-service/internal/app/services/core/auth"
	"konsulin-service/internal/app/services/core/roles"
	"konsulin-service/internal/app/services/core/users"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

// Version sets the default build version
var Version = "develop"

// Tag sets the default latest commit tag
var Tag = "0.0.1-rc"

func main() {
	driverConfig := config.NewDriverConfig()
	internalConfig := config.NewInternalConfig()

	log := logger.NewLogrusLogger(internalConfig)

	location, err := time.LoadLocation(internalConfig.App.Timezone)
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}
	time.Local = location
	log.Printf("Successfully set time base to %s", internalConfig.App.Timezone)

	mongoDB := database.NewMongoDB(driverConfig, log)
	redis := database.NewRedisClient(driverConfig, log)
	chiRouter := chi.NewRouter()

	bootstrapingTheApp(config.Bootstrap{
		Router:         chiRouter,
		MongoDB:        mongoDB,
		Redis:          redis,
		Logger:         log,
		DriverConfig:   driverConfig,
		InternalConfig: internalConfig,
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", internalConfig.App.Address, internalConfig.App.Port),
		Handler: chiRouter,
	}

	go func() {
		log.Printf("Server is running on port: %s", internalConfig.App.Port)
		log.Printf("Server Version: %s", Version)
		log.Printf("Server Tag: %s", Tag)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c

	logrus.Println("Waiting for pending requests that already received by server to be processed..")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Second*time.Duration(internalConfig.App.ShutdownTimeout),
	)
	defer cancel()
	for i := internalConfig.App.ShutdownTimeout; i > 0; i-- {
		time.Sleep(1 * time.Second)
		log.Printf("Shutting down in %d...", i)
	}

	// Shutdown the server
	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	<-shutdownCtx.Done()

	log.Println("Server exiting")
}

func bootstrapingTheApp(bootstrap config.Bootstrap) {
	// Redis
	redisRepository := redis.NewRedisRepository(bootstrap.Redis)

	// Patient
	patientFhirClient := patients.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePatient)

	// Practitioner
	practitionerFhirClient := practitioners.NewPractitionerFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePractitioner)

	// User
	userMongoRepository := users.NewUserMongoRepository(bootstrap.MongoDB, bootstrap.DriverConfig.MongoDB.DbName)
	userUseCase := users.NewUserUsecase(userMongoRepository, patientFhirClient, practitionerFhirClient)
	userController := users.NewUserController(userUseCase)

	roleMongoRepository := roles.NewRoleMongoRepository(bootstrap.MongoDB, bootstrap.DriverConfig.MongoDB.DbName)

	// Auth
	authUseCase, err := auth.NewAuthUsecase(userMongoRepository, redisRepository, roleMongoRepository, patientFhirClient, practitionerFhirClient, bootstrap.InternalConfig)
	if err != nil {
		bootstrap.Logger.Fatalln(err)
	}
	authController := auth.NewAuthController(authUseCase)

	// Middlewares
	middlewares := middlewares.NewMiddlewares(authUseCase, bootstrap.InternalConfig)

	routers.SetupRoutes(bootstrap.Router, bootstrap.InternalConfig, bootstrap.Logger, middlewares, userController, authController)
}
