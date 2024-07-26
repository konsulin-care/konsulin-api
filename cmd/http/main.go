package main

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/database"
	"konsulin-service/internal/app/drivers/logger"
	"konsulin-service/internal/app/drivers/mailer"
	"konsulin-service/internal/app/services/core/auth"
	educationLevels "konsulin-service/internal/app/services/core/education_levels"
	"konsulin-service/internal/app/services/core/genders"
	"konsulin-service/internal/app/services/core/roles"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/core/users"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	redisKonsulin "konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/app/services/shared/smtp"
	"konsulin-service/internal/pkg/constvars"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Version sets the default build version
var Version = "develop"

// Tag sets the default latest commit tag
var Tag = "0.0.1-rc"

type Bootstrap struct {
	Router         *chi.Mux
	MongoDB        *mongo.Database
	Redis          *redis.Client
	Logger         *zap.Logger
	SMTP           *mailer.SMTPClient
	DriverConfig   *config.DriverConfig
	InternalConfig *config.InternalConfig
}

func main() {
	driverConfig := config.NewDriverConfig()
	internalConfig := config.NewInternalConfig()

	logger := logger.NewZapLogger(driverConfig, internalConfig)
	defer logger.Sync()

	location, err := time.LoadLocation(internalConfig.App.Timezone)
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}
	time.Local = location
	log.Printf("Successfully set time base to %s", internalConfig.App.Timezone)

	mongoDB := database.NewMongoDB(driverConfig)
	redis := database.NewRedisClient(driverConfig)
	smtpClient := mailer.NewSMTPClient(driverConfig)
	chiRouter := chi.NewRouter()

	err = bootstrapingTheApp(Bootstrap{
		Router:         chiRouter,
		MongoDB:        mongoDB,
		Redis:          redis,
		Logger:         logger,
		SMTP:           smtpClient,
		DriverConfig:   driverConfig,
		InternalConfig: internalConfig,
	})

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

func bootstrapingTheApp(bootstrap Bootstrap) error {
	// Drivers are splitted into: 'service' related to functionality and 'repository' related to Data Access Layer
	// All deps in /internal/app/shared initiated here before injected into usecases
	redisRepository := redisKonsulin.NewRedisRepository(bootstrap.Redis)
	smtpService := smtp.NewSmtpService(bootstrap.SMTP)
	sessionService := session.NewSessionService(redisRepository)

	// Patient
	patientFhirClient := patients.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePatient)

	// Practitioner
	practitionerFhirClient := practitioners.NewPractitionerFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePractitioner)

	// User
	userMongoRepository := users.NewUserMongoRepository(bootstrap.MongoDB, bootstrap.DriverConfig.MongoDB.DbName)
	userUseCase := users.NewUserUsecase(userMongoRepository, patientFhirClient, practitionerFhirClient, redisRepository, sessionService)
	userController := users.NewUserController(bootstrap.Logger, userUseCase)

	// Education Level
	educationLevelMongoRepository := educationLevels.NewEducationLevelMongoRepository(bootstrap.MongoDB, bootstrap.DriverConfig.MongoDB.DbName)
	educationLevelUseCase, err := educationLevels.NewEducationLevelUsecase(educationLevelMongoRepository, redisRepository)
	if err != nil {
		return err
	}
	educationLevelController := educationLevels.NewEducationLevelController(bootstrap.Logger, educationLevelUseCase)

	// Gender
	genderMongoRepository := genders.NewGenderMongoRepository(bootstrap.MongoDB, bootstrap.DriverConfig.MongoDB.DbName)
	genderUseCase, err := genders.NewGenderUsecase(genderMongoRepository, redisRepository)
	if err != nil {
		return err
	}
	genderController := genders.NewGenderController(bootstrap.Logger, genderUseCase)

	// Role
	roleMongoRepository := roles.NewRoleMongoRepository(bootstrap.MongoDB, bootstrap.DriverConfig.MongoDB.DbName)

	// Auth
	authUseCase, err := auth.NewAuthUsecase(
		userMongoRepository,
		redisRepository,
		sessionService,
		roleMongoRepository,
		patientFhirClient,
		practitionerFhirClient,
		smtpService,
		bootstrap.InternalConfig,
	)

	if err != nil {
		return err
	}
	authController := auth.NewAuthController(bootstrap.Logger, authUseCase)

	// Middlewares
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
