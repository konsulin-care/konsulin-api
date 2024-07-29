package main

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/database"
	"konsulin-service/internal/app/drivers/logger"
	"konsulin-service/internal/app/drivers/messaging"
	"konsulin-service/internal/app/drivers/storage"
	"konsulin-service/internal/app/services/core/auth"
	educationLevels "konsulin-service/internal/app/services/core/education_levels"
	"konsulin-service/internal/app/services/core/genders"
	"konsulin-service/internal/app/services/core/roles"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/core/users"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/mailer"
	redisKonsulin "konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

// Version sets the default build version
var Version = "develop"

// Tag sets the default latest commit tag
var Tag = "0.0.1-rc"

func main() {
	driverConfig := config.NewDriverConfig()
	internalConfig := config.NewInternalConfig()

	logger := logger.NewZapLogger(driverConfig, internalConfig)

	location, err := time.LoadLocation(internalConfig.App.Timezone)
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}
	time.Local = location
	log.Printf("Successfully set time base to %s", internalConfig.App.Timezone)

	mongoDB := database.NewMongoDB(driverConfig)
	redis := database.NewRedisClient(driverConfig)
	rabbitMQ := messaging.NewRabbitMQ(driverConfig)
	minio := storage.NewMinio(driverConfig)
	log.Println(minio)

	chiRouter := chi.NewRouter()

	bootstrap := config.Bootstrap{
		Router:         chiRouter,
		MongoDB:        mongoDB,
		Redis:          redis,
		Logger:         logger,
		RabbitMQ:       rabbitMQ,
		InternalConfig: internalConfig,
	}

	// Now that we already init all the infrastructure
	// No need to pass the 'driverConfig' to Bootstrap anymore
	err = bootstrapingTheApp(bootstrap)

	if err != nil {
		log.Fatalf("Error while bootstraping the app: %s", err.Error())
	}

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

	log.Println("Waiting for pending requests that already received by server to be processed..")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Second*time.Duration(internalConfig.App.ShutdownTimeoutInSecond),
	)
	defer cancel()
	for i := internalConfig.App.ShutdownTimeoutInSecond; i > 0; i-- {
		time.Sleep(1 * time.Second)
		log.Printf("Shutting down in %d...", i)
	}

	// Shutdown the server
	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Shutdown the bootstrap components
	err = bootstrap.Shutdown(shutdownCtx)
	if err != nil {
		log.Fatalf("Error during shutdown: %v", err)
	}

	<-shutdownCtx.Done()

	log.Println("Server exiting")
}

func bootstrapingTheApp(bootstrap config.Bootstrap) error {
	// Drivers are splitted into: 'service' related to functionality and 'repository' related to Data Access Layer
	// All deps in /internal/app/shared initiated here before injected into usecases
	redisRepository := redisKonsulin.NewRedisRepository(bootstrap.Redis)
	mailerService, err := mailer.NewMailerService(bootstrap.RabbitMQ, bootstrap.InternalConfig.RabbitMQ.MailerQueue)
	if err != nil {
		return err
	}
	sessionService := session.NewSessionService(redisRepository)

	// All deps in /internal/app/core
	// Patient
	patientFhirClient := patients.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePatient)

	// Practitioner
	practitionerFhirClient := practitioners.NewPractitionerFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePractitioner)

	// User
	userMongoRepository := users.NewUserMongoRepository(bootstrap.MongoDB, bootstrap.InternalConfig.MongoDB.KonsulinDBName)
	userUseCase := users.NewUserUsecase(userMongoRepository, patientFhirClient, practitionerFhirClient, redisRepository, sessionService)
	userController := users.NewUserController(bootstrap.Logger, userUseCase)

	// Education Level
	educationLevelMongoRepository := educationLevels.NewEducationLevelMongoRepository(bootstrap.MongoDB, bootstrap.InternalConfig.MongoDB.KonsulinDBName)
	educationLevelUseCase, err := educationLevels.NewEducationLevelUsecase(educationLevelMongoRepository, redisRepository)
	if err != nil {
		return err
	}
	educationLevelController := educationLevels.NewEducationLevelController(bootstrap.Logger, educationLevelUseCase)

	// Gender
	genderMongoRepository := genders.NewGenderMongoRepository(bootstrap.MongoDB, bootstrap.InternalConfig.MongoDB.KonsulinDBName)
	genderUseCase, err := genders.NewGenderUsecase(genderMongoRepository, redisRepository)
	if err != nil {
		return err
	}
	genderController := genders.NewGenderController(bootstrap.Logger, genderUseCase)

	// Role
	roleMongoRepository := roles.NewRoleMongoRepository(bootstrap.MongoDB, bootstrap.InternalConfig.MongoDB.KonsulinDBName)

	// Auth
	authUseCase, err := auth.NewAuthUsecase(
		userMongoRepository,
		redisRepository,
		sessionService,
		roleMongoRepository,
		patientFhirClient,
		practitionerFhirClient,
		mailerService,
		bootstrap.InternalConfig,
	)

	if err != nil {
		return err
	}
	authController := auth.NewAuthController(bootstrap.Logger, authUseCase)

	// All Middlewares
	middlewares := middlewares.NewMiddlewares(bootstrap.Logger, sessionService, authUseCase, bootstrap.InternalConfig)

	routers.SetupRoutes(
		bootstrap.Router,
		bootstrap.InternalConfig,
		middlewares,
		userController,
		authController,
		educationLevelController,
		genderController,
	)

	return nil
}
