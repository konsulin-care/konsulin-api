package main

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/database"
	"konsulin-service/internal/app/drivers/logger"
	"konsulin-service/internal/app/drivers/messaging"
	"konsulin-service/internal/app/drivers/storage"
	"konsulin-service/internal/app/services/core/auth"
	"konsulin-service/internal/app/services/core/organization"
	"konsulin-service/internal/app/services/core/payments"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/core/slot"
	"konsulin-service/internal/app/services/core/transactions"
	"konsulin-service/internal/app/services/core/users"
	"konsulin-service/internal/app/services/core/webhook"
	bundle "konsulin-service/internal/app/services/fhir_spark/bundle"
	invoicesFhir "konsulin-service/internal/app/services/fhir_spark/invoices"
	organizationsFhir "konsulin-service/internal/app/services/fhir_spark/organizations"
	patientsFhir "konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/persons"
	practitionerRoleFhir "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	scheduleFhir "konsulin-service/internal/app/services/fhir_spark/schedules"
	"konsulin-service/internal/app/services/fhir_spark/service_requests"
	slotFhir "konsulin-service/internal/app/services/fhir_spark/slots"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/app/services/shared/locker"
	"konsulin-service/internal/app/services/shared/mailer"
	"konsulin-service/internal/app/services/shared/payment_gateway"
	"konsulin-service/internal/app/services/shared/ratelimiter"
	redisKonsulin "konsulin-service/internal/app/services/shared/redis"
	storageKonsulin "konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/app/services/shared/webhookqueue"
	"konsulin-service/internal/app/services/shared/whatsapp"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	xendit "github.com/xendit/xendit-go/v7"
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
		Redis:          redis,
		Logger:         logger,
		Minio:          minio,
		RabbitMQ:       rabbitMQ,
		InternalConfig: internalConfig,
		DriverConfig:   driverConfig,
	}

	// Initialize the application with the bootstrap components
	err = bootstrapingTheApp(&bootstrap)
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
func bootstrapingTheApp(bootstrap *config.Bootstrap) error {
	// Initialize the repository for Redis
	redisRepository := redisKonsulin.NewRedisRepository(bootstrap.Redis, bootstrap.Logger)

	// Initialize the mailer service with RabbitMQ
	mailerService, err := mailer.NewMailerService(bootstrap.RabbitMQ, bootstrap.Logger, bootstrap.InternalConfig.RabbitMQ.MailerQueue)
	if err != nil {
		return err
	}

	// Initialize the whatsApp service with RabbitMQ
	whatsAppService, err := whatsapp.NewWhatsAppService(bootstrap.RabbitMQ, bootstrap.Logger, bootstrap.InternalConfig.RabbitMQ.WhatsAppQueue)
	if err != nil {
		return err
	}

	// Initialize oy service (kept for backward-compatibility; not used for creation)
	_ = payment_gateway.NewOyService(bootstrap.InternalConfig, bootstrap.Logger)

	// Initialize Xendit client (reusable)
	xenditClient := xendit.NewClient(bootstrap.InternalConfig.Xendit.APIKey)

	// Initialize Minio storage
	minioStorage := storageKonsulin.NewMinioStorage(bootstrap.Minio)

	// Initialize session service with Redis repository
	sessionService := session.NewSessionService(redisRepository, bootstrap.Logger)

	// Initialize session service with Redis repository
	lockService := locker.NewLockService(redisRepository, bootstrap.Logger)

	// Initialize FHIR clients
	patientFhirClient := patientsFhir.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	practitionerFhirClient := practitioners.NewPractitionerFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	practitionerRoleClient := practitionerRoleFhir.NewPractitionerRoleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	personFhirClient := persons.NewPersonFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	organizationFhirClient := organizationsFhir.NewOrganizationFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	scheduleClient := scheduleFhir.NewScheduleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	slotClient := slotFhir.NewSlotFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	serviceRequestFhirClient := service_requests.NewServiceRequestFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)

	jwtManager, err := jwtmanager.NewJWTManager(bootstrap.InternalConfig, bootstrap.Logger)
	if err != nil {
		return err
	}

	// Ensure default FHIR Groups exist for ServiceRequest subjects
	_ = serviceRequestFhirClient.EnsureAllNecessaryGroupsExists(context.Background())

	userUsecase := users.NewUserUsecase(
		nil, // userMongoRepository, not used yet
		patientFhirClient,
		practitionerFhirClient,
		personFhirClient,
		practitionerRoleClient,
		nil, // organizationFhirClient, not used yet
		redisRepository,
		sessionService,
		minioStorage,
		bootstrap.InternalConfig,
		bootstrap.Logger,
		lockService,
		jwtManager,
	)

	// Initialize Auth usecase with dependencies
	authUseCase, err := auth.NewAuthUsecase(
		redisRepository,
		sessionService,
		patientFhirClient,
		practitionerFhirClient,
		userUsecase,
		mailerService,
		whatsAppService,
		minioStorage,
		bootstrap.InternalConfig,
		bootstrap.DriverConfig,
		bootstrap.Logger,
	)
	if err != nil {
		return err
	}
	authController := controllers.NewAuthController(bootstrap.Logger, authUseCase)

	// Initialize middlewares with logger, session service, and auth usecase
	middlewares := middlewares.NewMiddlewares(
		bootstrap.Logger,
		sessionService,
		authUseCase,
		bootstrap.InternalConfig,
		practitionerFhirClient,
		patientFhirClient,
		practitionerRoleClient,
		scheduleClient,
	)

	// Initialize supertokens
	err = authUseCase.InitializeSupertoken()
	if err != nil {
		log.Fatalf("Error initializing supertokens: %v", err)
	}

	// Initialize webhook components
	webhookLimiter := ratelimiter.NewHookRateLimiter(redisRepository, bootstrap.Logger, bootstrap.InternalConfig)
	webhookQueueService, err := webhookqueue.NewService(bootstrap.RabbitMQ, bootstrap.Logger, bootstrap.InternalConfig.Webhook.MaxQueue)
	if err != nil {
		return err
	}
	webhookUsecase := webhook.NewUsecase(bootstrap.Logger, bootstrap.InternalConfig, webhookQueueService, jwtManager, patientFhirClient, practitionerFhirClient, serviceRequestFhirClient)
	webhookController := controllers.NewWebhookController(bootstrap.Logger, webhookUsecase, webhookLimiter, bootstrap.InternalConfig)
	// Initialize payment usecase and controller (inject JWT manager)
	serviceRequestStorage := storageKonsulin.NewServiceRequestStorage(serviceRequestFhirClient, bootstrap.Logger)
	invoiceFhirClient := invoicesFhir.NewInvoiceFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)

	bundleClient := bundle.NewBundleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	slotUsecase := slot.NewSlotUsecase(scheduleClient, lockService, slotClient, practitionerRoleClient, practitionerFhirClient, personFhirClient, bundleClient, bootstrap.InternalConfig, bootstrap.Logger)

	paymentUsecase := payments.NewPaymentUsecase(
		transactions.NewTransactionPostgresRepository(nil, bootstrap.Logger),
		bootstrap.InternalConfig,
		jwtManager,
		patientFhirClient,
		practitionerFhirClient,
		personFhirClient,
		serviceRequestStorage,
		payment_gateway.NewOyService(bootstrap.InternalConfig, bootstrap.Logger),
		xenditClient,
		invoiceFhirClient,
		practitionerRoleClient,
		slotClient,
		scheduleClient,
		bundleClient,
		slotUsecase,
		bootstrap.Logger,
	)
	paymentController := controllers.NewPaymentController(bootstrap.Logger, paymentUsecase)

	scheduleController := controllers.NewScheduleController(slotUsecase, bootstrap.Logger)

	// Initialize organization usecase and controller
	orgUsecase := organization.NewOrganizationUsecase(
		practitionerFhirClient,
		personFhirClient,
		organizationFhirClient,
		bundleClient,
		bootstrap.InternalConfig,
		bootstrap.Logger,
	)
	orgController := controllers.NewOrganizationController(bootstrap.Logger, orgUsecase)

	orgUsecase.InitializeKonsulinOrganizationResource(context.Background())

	// Start webhook worker ticker (best-effort lock ensures single execution)
	worker := webhook.NewWorker(bootstrap.Logger, bootstrap.InternalConfig, lockService, webhookQueueService, jwtManager)
	stopWorker := worker.Start(context.Background())
	bootstrap.WorkerStop = stopWorker

	// Start slot top-up worker (leader lock inside)
	slotWorker := slot.NewWorker(bootstrap.Logger, bootstrap.InternalConfig, lockService, practitionerRoleClient, slotUsecase)
	slotWorker.Start(context.Background())
	bootstrap.SlotWorkerStop = slotWorker.Stop

	// Setup routes with the router, configuration, middlewares, and controllers
	routers.SetupRoutes(
		bootstrap.Router,
		bootstrap.InternalConfig,
		bootstrap.Logger,
		middlewares,
		authController,
		paymentController,
		webhookController,
		scheduleController,
		orgController,
	)

	return nil
}
