package main

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/database"
	"konsulin-service/internal/app/drivers/logger"
	"konsulin-service/internal/app/drivers/webframework"
	"konsulin-service/internal/app/services/auth"
	"konsulin-service/internal/app/services/patients"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/app/services/users"
	"konsulin-service/internal/pkg/constvars"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	driverConfig := config.NewDriverConfig()
	internalConfig := config.NewInternalConfig()
	logger.InitLogger(driverConfig)

	fiberFramework := webframework.NewFiber(driverConfig)
	mongoDB := database.NewMongoDB(driverConfig)
	redis := database.NewRedisClient(driverConfig)

	bootstrapingTheApp(config.Bootstrap{
		App:            fiberFramework,
		MongoDB:        mongoDB,
		Redis:          redis,
		DriverConfig:   driverConfig,
		InternalConfig: internalConfig,
	})

	go func() {
		if err := fiberFramework.Listen(driverConfig.App.Port); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c

	log.Println("Waiting for pending requests that already received by server to be processed..")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Second*time.Duration(driverConfig.App.ShutdownTimeout),
	)
	defer cancel()
	for i := driverConfig.App.ShutdownTimeout; i > 0; i-- {
		time.Sleep(1 * time.Second)
		log.Printf("Shutting down in %d...", i)
	}

	// Shutdown the server
	if err := fiberFramework.Shutdown(); err != nil {
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

	// User
	userMongoRepository := users.NewUserMongoRepository(
		bootstrap.MongoDB,
		bootstrap.DriverConfig.MongoDB.DbName,
	)

	// Patient
	patientMongoRepository := patients.NewPatientMongoRepository(
		bootstrap.MongoDB,

		bootstrap.DriverConfig.MongoDB.DbName,
	)
	patientFhirClient := patients.NewPatientFhirClient(bootstrap.InternalConfig.FHIR.BaseUrl + constvars.ResourcePatient)
	patientUsecase := patients.NewPatientUsecase(patientMongoRepository, patientFhirClient, userMongoRepository)
	patientController := patients.NewPatientController(patientUsecase)

	// Auth
	authUseCase := auth.NewAuthUsecase(patientMongoRepository, userMongoRepository, redisRepository, patientFhirClient, bootstrap.InternalConfig)
	authController := auth.NewAuthController(authUseCase)

	routers.SetupRoutes(bootstrap.App, bootstrap.DriverConfig.App, middlewares, patientController, authController)
}
