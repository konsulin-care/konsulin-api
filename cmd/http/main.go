package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/database"
	"konsulin-service/internal/app/drivers/logger"
	"konsulin-service/internal/app/services/core/auth"
	"konsulin-service/internal/app/services/core/organization"
	"konsulin-service/internal/app/services/core/payments"
	"konsulin-service/internal/app/services/core/services"
	"konsulin-service/internal/app/services/core/slot"
	"konsulin-service/internal/app/services/core/users"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/fhir_spark/service_requests"
	"konsulin-service/internal/app/services/shared/locker"
	"konsulin-service/internal/pkg/buildinfo"

	privacy "konsulin-service/internal/app/services/core/privacy"

	bundle "konsulin-service/internal/app/services/fhir_spark/bundle"
	invoicesFhir "konsulin-service/internal/app/services/fhir_spark/invoices"
	organizationsFhir "konsulin-service/internal/app/services/fhir_spark/organizations"
	patientsFhir "konsulin-service/internal/app/services/fhir_spark/patients"

	planDefinitionsFhir "konsulin-service/internal/app/services/fhir_spark/plan_definitions"
	practitionerRoleFhir "konsulin-service/internal/app/services/fhir_spark/practitioner_role"

	privacyFhir "konsulin-service/internal/app/services/fhir_spark/privacy"
	questionnaireResponsesFhir "konsulin-service/internal/app/services/fhir_spark/questionnaire_responses"
	scheduleFhir "konsulin-service/internal/app/services/fhir_spark/schedules"

	slotFhir "konsulin-service/internal/app/services/fhir_spark/slots"

	redisKonsulin "konsulin-service/internal/app/services/shared/redis"

	"github.com/go-chi/chi/v5"
)

func main() {
	// Load configuration for external drivers (database, redis, etc.)
	driverConfig := config.NewDriverConfig()

	// Load internal application configuration
	internalConfig := config.NewInternalConfig()

	// Initialize the logger
	logger, err := logger.NewZapLogger(driverConfig, internalConfig)
	if err != nil {
		log.Fatalf("Error initializing zap logger: %v", err)
	}

	// Set the application's timezone
	location, err := time.LoadLocation(internalConfig.App.Timezone)
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}
	time.Local = location
	log.Printf("Successfully set time base to %s", internalConfig.App.Timezone)

	// Initialize Redis connection
	redis, err := database.NewRedisClient(driverConfig)
	if err != nil {
		log.Fatalf("Error connecting to Redis: %s", err.Error())
	}

	// Create a new router
	chiRouter := chi.NewRouter()

	// Bundle all initialized components into a Bootstrap struct
	bootstrap := config.Bootstrap{
		Router:         chiRouter,
		Redis:          redis,
		Logger:         logger,
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
		Addr:              fmt.Sprintf("%s:%s", internalConfig.App.Address, internalConfig.App.Port),
		Handler:           chiRouter,
		ReadHeaderTimeout: 60 * time.Second,
	}

	// Start the server in a separate goroutine
	go func() {
		log.Printf("Server Version: %s", buildinfo.Version)
		log.Printf("Server Tag: %s", buildinfo.Tag)
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

	log.Printf("Shutting down server gracefully (timeout: %ds)", internalConfig.App.ShutdownTimeoutInSeconds)

	// Shutdown the HTTP server gracefully
	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		cancel()
		return
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
	// Build the gateway federation components before the usecases so later
	// wiring (enrollment, inbound handlers) can depend on the adapter.
	gwWiring, err := newGateway(bootstrap)
	if err != nil {
		return fmt.Errorf("error initializing gateway: %w", err)
	}

	// Initialize the repository for Redis
	redisRepository := redisKonsulin.NewRedisRepository(bootstrap.Redis, bootstrap.Logger)

	lockService := locker.NewLockService(redisRepository, bootstrap.Logger)

	// Initialize FHIR clients
	patientFhirClient := patientsFhir.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	practitionerFhirClient := practitioners.NewPractitionerFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	practitionerRoleClient := practitionerRoleFhir.NewPractitionerRoleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	organizationFhirClient := organizationsFhir.NewOrganizationFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	scheduleClient := scheduleFhir.NewScheduleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	slotClient := slotFhir.NewSlotFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	serviceRequestFhirClient := service_requests.NewServiceRequestFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	questionnaireResponseFhirClient := questionnaireResponsesFhir.NewQuestionnaireResponseFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	planDefinitionFhirClient := planDefinitionsFhir.NewPlanDefinitionFinder(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)

	// T39: magic links travel through the gateway send-magiclink listing on
	// the automation party, using the adapter built early by newGateway.
	var magicLinkGateway auth.GatewayWebhookClient
	if gwWiring != nil && gwWiring.client != nil {
		magicLinkGateway = gwWiring.client
	}
	magicLinkDelivery := auth.NewGatewayMagicLinkSender(magicLinkGateway, bootstrap.InternalConfig.Gateway)

	// Ensure default FHIR Groups exist for ServiceRequest subjects
	_ = serviceRequestFhirClient.EnsureAllNecessaryGroupsExists(context.Background())

	// T42A/T42B: optional gateway deps for the omnichannel profile sync.
	// The legacy forwarder/HTTP paths are removed (T42B): a nil client, a
	// disabled gateway or an empty APP_AUTOMATION_PARTY_ID skips the sync
	// with a log.
	var gatewayWebhooks users.GatewayWebhookClient
	if gwWiring != nil && gwWiring.client != nil {
		gatewayWebhooks = gwWiring.client
	}

	// T44: signup identity registration travels through the gateway
	// identity plane. The adapter is injected even when disabled: the
	// usecase skips the registration in standalone mode (no I/O) and a
	// missing wiring (nil) skips it the same way.
	var identityClient users.IdentityClient
	if gwWiring != nil && gwWiring.client != nil {
		identityClient = gwWiring.client
	}

	userUsecase := users.NewUserFHIRInitializer(users.UserFHIRInitializerDeps{
		PatientFhirClient:          patientFhirClient,
		PractitionerFhirClient:     practitionerFhirClient,
		PractitionerRoleFhirClient: practitionerRoleClient,
		OrganizationFhirClient:     nil, // not used yet
		RedisRepository:            redisRepository,
		InternalConfig:             bootstrap.InternalConfig,
		Logger:                     bootstrap.Logger,
		LockerService:              lockService,
		GatewayWebhooks:            gatewayWebhooks,
		GatewayAutomationPartyID:   bootstrap.InternalConfig.Gateway.AutomationPartyID,
		IdentityClient:             identityClient,
	})

	bundleClient := bundle.NewBundleFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)

	// Initialize Auth usecase with dependencies
	authUseCase, err := auth.NewAuthUsecase(
		redisRepository,
		patientFhirClient,
		practitionerFhirClient,
		questionnaireResponseFhirClient,
		bundleClient,
		userUsecase,
		magicLinkDelivery,
		bootstrap.InternalConfig,
		bootstrap.DriverConfig,
		bootstrap.Logger,
	)
	if err != nil {
		return err
	}
	authController := controllers.NewAuthController(bootstrap.Logger, authUseCase, bootstrap.InternalConfig)

	// Initialize middlewares
	middlewares := middlewares.NewMiddlewares(
		bootstrap.Logger,
		authUseCase,
		bootstrap.InternalConfig,
		practitionerFhirClient,
		patientFhirClient,
		practitionerRoleClient,
		scheduleClient,
		questionnaireResponseFhirClient,
		planDefinitionFhirClient,
	)

	// Initialize supertokens
	err = authUseCase.InitializeSupertoken()
	if err != nil {
		return fmt.Errorf("error initializing supertokens: %w", err)
	}

	// Initialize payment usecase and controller
	invoiceFhirClient := invoicesFhir.NewInvoiceFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)

	slotUsecase := slot.NewSlotUsecase(scheduleClient, lockService, slotClient, practitionerRoleClient, practitionerFhirClient, bundleClient, bootstrap.InternalConfig, bootstrap.Logger)

	// Gateway deps for the payment usecase, built from the gateway federation
	// clients plus the provider resolver bound to the configured aliases.
	providerResolver := services.NewProviderResolver(&bootstrap.InternalConfig.Gateway)
	gatewayDeps := payments.GatewayDeps{
		ResolveProvider:   providerResolver.ResolveProvider,
		OwnPartyID:        bootstrap.InternalConfig.Gateway.PartyID,
		AutomationPartyID: bootstrap.InternalConfig.Gateway.AutomationPartyID,
	}
	if gwWiring != nil && gwWiring.client != nil {
		gatewayDeps.Payments = gwWiring.client
		gatewayDeps.Webhooks = gwWiring.client
		gatewayDeps.Listings = gwWiring.client
	}

	paymentUsecase := payments.NewPaymentUsecase(
		bootstrap.InternalConfig,
		patientFhirClient,
		practitionerFhirClient,
		invoiceFhirClient,
		practitionerRoleClient,
		slotClient,
		scheduleClient,
		bundleClient,
		slotUsecase,
		gatewayDeps,
		bootstrap.Logger,
	)
	paymentController := controllers.NewPaymentController(bootstrap.Logger, paymentUsecase)

	scheduleController := controllers.NewScheduleController(slotUsecase, bootstrap.Logger)

	// Initialize organization usecase and controller
	orgUsecase := organization.NewOrganizationUsecase(
		practitionerFhirClient,
		practitionerRoleClient,
		organizationFhirClient,
		bundleClient,
		bootstrap.InternalConfig,
		bootstrap.Logger,
	)
	orgController := controllers.NewOrganizationController(bootstrap.Logger, orgUsecase)

	// Initialize purge (erasure) components: FHIR client, account deletion, usecase, controller.
	purgeFhirClient := privacyFhir.NewPurgeFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl, bootstrap.Logger)
	accountDeletion := users.NewUserAccountDeleter(bootstrap.Logger)
	purgeUsecase := privacy.NewPatientDataPurger(purgeFhirClient, bundleClient, accountDeletion, bootstrap.Logger)
	purgeController := controllers.NewPurgeController(purgeUsecase, middlewares, bootstrap.Logger)

	if err := orgUsecase.InitializeKonsulinOrganizationResource(context.Background()); err != nil {
		return fmt.Errorf("error initializing Konsulin organization resource: %w", err)
	}

	// Publish the gateway status on the controller set (starting background
	// enrollment when APP_GATEWAY_URL is configured) before routes register,
	// so federation-dependent routes see the status.
	ctrls := routers.Controllers{
		Auth:         authController,
		Payment:      paymentController,
		Schedule:     scheduleController,
		Organization: orgController,
		Purge:        purgeController,
	}
	registerFederation(bootstrap, &ctrls, bootstrap.InternalConfig.Gateway, paymentUsecase)

	// T71: research databank — SQLite store plus the nightly 00:00 local
	// extraction scheduler (no-op unless APP_RESEARCH_PSEUDONYM_KEY is set).
	if err := registerResearch(bootstrap); err != nil {
		return fmt.Errorf("error initializing research databank: %w", err)
	}

	// Setup routes with the router, configuration, middlewares, and controllers
	routers.SetupRoutes(
		bootstrap.Router,
		bootstrap.InternalConfig,
		bootstrap.Logger,
		middlewares,
		ctrls,
	)

	return nil
}
