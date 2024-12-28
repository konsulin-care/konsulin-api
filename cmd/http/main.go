package main

import (
	"context"
	"fmt"
	"konsulin-service/cmd/migration"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/database"
	"konsulin-service/internal/app/drivers/logger"
	"konsulin-service/internal/app/drivers/messaging"
	"konsulin-service/internal/app/drivers/storage"
	"konsulin-service/internal/app/services/core/appointments"
	assessmentResponses "konsulin-service/internal/app/services/core/assessment_responses"
	"konsulin-service/internal/app/services/core/assessments"
	"konsulin-service/internal/app/services/core/auth"
	"konsulin-service/internal/app/services/core/cities"
	"konsulin-service/internal/app/services/core/clinicians"
	"konsulin-service/internal/app/services/core/clinics"
	educationLevels "konsulin-service/internal/app/services/core/education_levels"
	"konsulin-service/internal/app/services/core/genders"
	"konsulin-service/internal/app/services/core/patients"
	"konsulin-service/internal/app/services/core/payments"
	"konsulin-service/internal/app/services/core/roles"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/core/transactions"
	"konsulin-service/internal/app/services/core/users"
	fhir_appointments "konsulin-service/internal/app/services/fhir_spark/appointments"
	"konsulin-service/internal/app/services/fhir_spark/charge_item_definitions"
	"konsulin-service/internal/app/services/fhir_spark/organizations"
	patientsFhir "konsulin-service/internal/app/services/fhir_spark/patients"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	questionnairesFhir "konsulin-service/internal/app/services/fhir_spark/questionnaires"
	questionnaireResponsesFhir "konsulin-service/internal/app/services/fhir_spark/questionnaires_responses"
	"konsulin-service/internal/app/services/fhir_spark/schedules"
	"konsulin-service/internal/app/services/fhir_spark/slots"
	"konsulin-service/internal/app/services/shared/mailer"
	"konsulin-service/internal/app/services/shared/payment_gateway"
	redisKonsulin "konsulin-service/internal/app/services/shared/redis"
	storageKonsulin "konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/app/services/shared/whatsapp"
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
	postgresDB := database.NewPostgresDB(driverConfig)

	migration.Run(postgresDB)

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
		PostgresDB:     postgresDB,
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
		log.Printf("Server Version: %s", Version)
		log.Printf("Server Tag: %s", Tag)
		log.Printf("Server is running on port: %s", internalConfig.App.Port)
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

	// Initialize the whatsApp service with RabbitMQ
	whatsAppService, err := whatsapp.NewWhatsAppService(bootstrap.RabbitMQ, bootstrap.InternalConfig.RabbitMQ.WhatsAppQueue)
	if err != nil {
		return err
	}

	// Initialize oy service
	oyService, err := payment_gateway.NewOyService(bootstrap.InternalConfig)
	if err != nil {
		return err
	}

	// Initialize Minio storage
	minioStorage := storageKonsulin.NewMinioStorage(bootstrap.Minio)

	// Initialize session service with Redis repository
	sessionService := session.NewSessionService(redisRepository)

	// Initialize FHIR clients
	patientFhirClient := patientsFhir.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	practitionerFhirClient := practitioners.NewPractitionerFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	organizationFhirClient := organizations.NewOrganizationFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	practitionerRoleFhirClient := practitionerRoles.NewPractitionerRoleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	scheduleFhirClient := schedules.NewScheduleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	slotFhirClient := slots.NewSlotFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	appointmentFhirClient := fhir_appointments.NewAppointmentFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	chargeItemDefinitionFhirClient := charge_item_definitions.NewChargeItemDefinitionFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	questionnaireFhirClient := questionnairesFhir.NewQuestionnaireFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)
	questionnaireResponseFhirClient := questionnaireResponsesFhir.NewQuestionnaireResponseFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl)

	// Initialize Users dependencies
	userPostgresRepository := users.NewUserPostgresRepository(bootstrap.PostgresDB)
	userUseCase := users.NewUserUsecase(userPostgresRepository, patientFhirClient, practitionerFhirClient, practitionerRoleFhirClient, organizationFhirClient, redisRepository, sessionService, minioStorage, bootstrap.InternalConfig)
	userController := controllers.NewUserController(bootstrap.Logger, userUseCase, bootstrap.InternalConfig)

	// Initialize Education Level dependencies
	educationLevelPostgresRepository := educationLevels.NewEducationLevelPostgresRepository(bootstrap.PostgresDB)
	educationLevelUseCase, err := educationLevels.NewEducationLevelUsecase(educationLevelPostgresRepository, redisRepository)
	if err != nil {
		return err
	}
	educationLevelController := controllers.NewEducationLevelController(bootstrap.Logger, educationLevelUseCase)

	// Initialize Gender dependencies
	genderPostgresRepository := genders.NewGenderPostgresRepository(bootstrap.PostgresDB)
	genderUseCase, err := genders.NewGenderUsecase(genderPostgresRepository, redisRepository)
	if err != nil {
		return err
	}
	genderController := controllers.NewGenderController(bootstrap.Logger, genderUseCase)

	// Initialize Role repository with MongoDB
	rolePostgresRepository := roles.NewRolePostgresRepository(bootstrap.PostgresDB)

	// Initialize Transaction repository with MongoDB
	transactionPostgresRepository := transactions.NewTransactionPostgresRepository(bootstrap.PostgresDB)

	// Initialize Clinic dependencies
	clinicUsecase := clinics.NewClinicUsecase(organizationFhirClient, practitionerRoleFhirClient, practitionerFhirClient, scheduleFhirClient, redisRepository, bootstrap.InternalConfig)
	clinicController := controllers.NewClinicController(bootstrap.Logger, clinicUsecase)

	// Initialize Clinic dependencies
	clinicianUsecase := clinicians.NewClinicianUsecase(practitionerFhirClient, practitionerRoleFhirClient, organizationFhirClient, scheduleFhirClient, slotFhirClient, appointmentFhirClient, chargeItemDefinitionFhirClient, sessionService)
	clinicianController := controllers.NewClinicianController(bootstrap.Logger, clinicianUsecase)

	// Initialize Clinic dependencies
	patientUsecase := patients.NewPatientUsecase(practitionerFhirClient, practitionerRoleFhirClient, scheduleFhirClient, slotFhirClient, appointmentFhirClient, sessionService, oyService, bootstrap.InternalConfig)
	patientController := controllers.NewPatientController(bootstrap.Logger, patientUsecase)

	// Initialize Assessment dependencies
	assessmentUsecase := assessments.NewAssessmentUsecase(questionnaireFhirClient)
	assessmentController := controllers.NewAssessmentController(bootstrap.Logger, assessmentUsecase)

	// Initialize Assessment Response dependencies
	assessmentResponseUsecase := assessmentResponses.NewAssessmentResponseUsecase(questionnaireResponseFhirClient, questionnaireFhirClient, patientFhirClient, redisRepository, bootstrap.InternalConfig)
	assessmentResponseController := controllers.NewAssessmentResponseController(bootstrap.Logger, assessmentResponseUsecase)

	// Initialize Assessment Response dependencies
	appointmentUsecase := appointments.NewAppointmentUsecase(transactionPostgresRepository, clinicianUsecase, appointmentFhirClient, patientFhirClient, practitionerFhirClient, slotFhirClient, redisRepository, sessionService, oyService, bootstrap.InternalConfig)
	appointmentController := controllers.NewAppointmentController(bootstrap.Logger, appointmentUsecase)

	// Initialize Auth usecase with dependencies
	authUseCase, err := auth.NewAuthUsecase(
		userPostgresRepository,
		redisRepository,
		sessionService,
		rolePostgresRepository,
		patientFhirClient,
		practitionerFhirClient,
		practitionerRoleFhirClient,
		questionnaireResponseFhirClient,
		mailerService,
		whatsAppService,
		minioStorage,
		bootstrap.InternalConfig,
	)
	if err != nil {
		return err
	}
	authController := controllers.NewAuthController(bootstrap.Logger, authUseCase)

	// Initialize Education Level dependencies
	cityPostgresRepository := cities.NewCityPostgresRepository(bootstrap.PostgresDB)
	cityUseCase, err := cities.NewCityUsecase(cityPostgresRepository, redisRepository)
	if err != nil {
		return err
	}
	cityController := controllers.NewCityController(bootstrap.Logger, cityUseCase)

	paymentUsecase := payments.NewPaymentUsecase(appointmentFhirClient, bootstrap.InternalConfig)
	paymentController := controllers.NewPaymentController(bootstrap.Logger, paymentUsecase)

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
		clinicianController,
		patientController,
		educationLevelController,
		cityController,
		genderController,
		assessmentController,
		assessmentResponseController,
		appointmentController,
		paymentController,
	)

	return nil
}
