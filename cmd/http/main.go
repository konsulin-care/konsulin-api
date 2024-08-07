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
	"konsulin-service/internal/app/services/core/clinics"
	educationLevels "konsulin-service/internal/app/services/core/education_levels"
	"konsulin-service/internal/app/services/core/genders"
	"konsulin-service/internal/app/services/core/roles"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/core/users"
	"konsulin-service/internal/app/services/fhir_spark/organizations"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/mailer"
	redisKonsulin "konsulin-service/internal/app/services/shared/redis"
	storageKonsulin "konsulin-service/internal/app/services/shared/storage"
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
	// Load configuration for external drivers (database, redis, etc.)
	driverConfig := config.NewDriverConfig()

	// Load internal application configuration
	internalConfig := config.NewInternalConfig()

	// Initialize the logger
	logger := logger.NewZapLogger(driverConfig, internalConfig)

	// Set the application's timezone
	location, err := time.LoadLocation(internalConfig.App.Timezone)
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}
	time.Local = location
	log.Printf("Successfully set time base to %s", internalConfig.App.Timezone)

	// Initialize MongoDB connection
	mongoDB := database.NewMongoDB(driverConfig)

	// Initialize Redis connection
	redis := database.NewRedisClient(driverConfig)

	// Initialize RabbitMQ connection
	rabbitMQ := messaging.NewRabbitMQ(driverConfig)

	// Initialize Minio client for storage
	minio := storage.NewMinio(driverConfig)

	// Create a new router
	chiRouter := chi.NewRouter()

	// Bundle all initialized components into a Bootstrap struct
	bootstrap := config.Bootstrap{
		Router:         chiRouter,
		MongoDB:        mongoDB,
		Redis:          redis,
		Logger:         logger,
		Minio:          minio,
		RabbitMQ:       rabbitMQ,
		InternalConfig: internalConfig,
	}

	// Initialize the application with the bootstrap components
	err = bootstrapingTheApp(bootstrap)
	if err != nil {
		log.Fatalf("Error while bootstrapping the app: %s", err.Error())
	}

	// Create an HTTP server with the router and address configuration
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", internalConfig.App.Address, internalConfig.App.Port),
		Handler: chiRouter,
	}

	// Start the server in a separate goroutine
	go func() {
		log.Printf("Server is running on port: %s", internalConfig.App.Port)
		log.Printf("Server Version: %s", Version)
		log.Printf("Server Tag: %s", Tag)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Setup a channel to listen for OS signals (e.g., interrupt or terminate)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until an OS signal is received
	<-c

	log.Println("Waiting for pending requests that already received by server to be processed..")

	// Create a context with a timeout for the shutdown process
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Second*time.Duration(internalConfig.App.ShutdownTimeoutInSeconds),
	)
	defer cancel()

	// Countdown for shutdown
	for i := internalConfig.App.ShutdownTimeoutInSeconds; i > 0; i-- {
		time.Sleep(1 * time.Second)
		log.Printf("Shutting down in %d...", i)
	}

	// Shutdown the HTTP server gracefully
	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Shutdown the bootstrap components gracefully
	err = bootstrap.Shutdown(shutdownCtx)
	if err != nil {
		log.Fatalf("Error during shutdown: %v", err)
	}

	// Wait for the shutdown context to be done
	<-shutdownCtx.Done()

	log.Println("Server exiting")
}

// bootstrapingTheApp initializes and sets up the application with the given bootstrap components
func bootstrapingTheApp(bootstrap config.Bootstrap) error {
	// Initialize the repository for Redis
	redisRepository := redisKonsulin.NewRedisRepository(bootstrap.Redis)

	// Initialize the mailer service with RabbitMQ
	mailerService, err := mailer.NewMailerService(bootstrap.RabbitMQ, bootstrap.InternalConfig.RabbitMQ.MailerQueue)
	if err != nil {
		return err
	}

	// Initialize Minio storage
	minioStorage := storageKonsulin.NewMinioStorage(bootstrap.Minio)

	// Initialize session service with Redis repository
	sessionService := session.NewSessionService(redisRepository)

	// Initialize FHIR clients
	patientFhirClient := patients.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePatient)
	practitionerFhirClient := practitioners.NewPractitionerFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePractitioner)
	organizationFhirClient := organizations.NewOrganizationFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourceOrganization)
	practitionerRoleFhirClient := practitionerRoles.NewPractitionerRoleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePractitionerRole)

	// Initialize Users dependencies
	userMongoRepository := users.NewUserMongoRepository(bootstrap.MongoDB, bootstrap.InternalConfig.MongoDB.KonsulinDBName)
	userUseCase := users.NewUserUsecase(userMongoRepository, patientFhirClient, practitionerFhirClient, redisRepository, sessionService, minioStorage, bootstrap.InternalConfig)
	userController := users.NewUserController(bootstrap.Logger, userUseCase)

	// Initialize Education Level dependencies
	educationLevelMongoRepository := educationLevels.NewEducationLevelMongoRepository(bootstrap.MongoDB, bootstrap.InternalConfig.MongoDB.KonsulinDBName)
	educationLevelUseCase, err := educationLevels.NewEducationLevelUsecase(educationLevelMongoRepository, redisRepository)
	if err != nil {
		return err
	}
	educationLevelController := educationLevels.NewEducationLevelController(bootstrap.Logger, educationLevelUseCase)

	// Initialize Gender dependencies
	genderMongoRepository := genders.NewGenderMongoRepository(bootstrap.MongoDB, bootstrap.InternalConfig.MongoDB.KonsulinDBName)
	genderUseCase, err := genders.NewGenderUsecase(genderMongoRepository, redisRepository)
	if err != nil {
		return err
	}
	genderController := genders.NewGenderController(bootstrap.Logger, genderUseCase)

	// Initialize Role repository with MongoDB
	roleMongoRepository := roles.NewRoleMongoRepository(bootstrap.MongoDB, bootstrap.InternalConfig.MongoDB.KonsulinDBName)

	// Initialize Clinic dependencies
	clinicUsecase := clinics.NewClinicUsecase(organizationFhirClient, practitionerRoleFhirClient, practitionerFhirClient, redisRepository, bootstrap.InternalConfig)
	clinicController := clinics.NewClinicController(bootstrap.Logger, clinicUsecase)

	// Initialize Auth usecase with dependencies
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

	// Initialize middlewares with logger, session service, and auth usecase
	middlewares := middlewares.NewMiddlewares(bootstrap.Logger, sessionService, authUseCase, bootstrap.InternalConfig)

	// Setup routes with the router, configuration, middlewares, and controllers
	routers.SetupRoutes(
		bootstrap.Router,
		bootstrap.InternalConfig,
		middlewares,
		userController,
		authController,
		clinicController,
		educationLevelController,
		genderController,
	)

	return nil
}
