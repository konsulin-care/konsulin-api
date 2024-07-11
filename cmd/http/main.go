package main

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/database"
	"konsulin-service/internal/app/drivers/logger"
	"konsulin-service/internal/app/services/auth"
	"konsulin-service/internal/app/services/patients"
	"konsulin-service/internal/app/services/practitioners"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/app/services/users"
	"konsulin-service/internal/pkg/constvars"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

func main() {
	driverConfig := config.NewDriverConfig()
	internalConfig := config.NewInternalConfig()

	log := logger.NewLogger(internalConfig)

	location, err := time.LoadLocation(internalConfig.App.Timezone)
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}
	time.Local = location

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
		Addr:    internalConfig.App.Port,
		Handler: chiRouter,
	}

	go func() {
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
	// for i := internalConfig.App.ShutdownTimeout; i > 0; i-- {
	// 	time.Sleep(1 * time.Second)
	// 	log.Printf("Shutting down in %d...", i)
	// }

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

	// Middlewares
	middlewares := middlewares.NewMiddlewares(redisRepository, bootstrap.InternalConfig)

	// Patient
	patientFhirClient := patients.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePatient)

	// Practitioner
	practitionerFhirClient := practitioners.NewPractitionerFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePractitioner)

	// User
	userMongoRepository := users.NewUserMongoRepository(
		bootstrap.MongoDB,
		bootstrap.DriverConfig.MongoDB.DbName,
	)
	userUseCase := users.NewUserUsecase(userMongoRepository, patientFhirClient)
	userController := users.NewUserController(userUseCase)

	// Auth
	authUseCase := auth.NewAuthUsecase(userMongoRepository, redisRepository, patientFhirClient, practitionerFhirClient, bootstrap.InternalConfig)
	authController := auth.NewAuthController(authUseCase)

	routers.SetupRoutes(bootstrap.Router, bootstrap.InternalConfig, bootstrap.Logger, middlewares, userController, authController)
}
