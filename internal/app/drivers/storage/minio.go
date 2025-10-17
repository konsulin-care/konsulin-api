package storage

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func NewMinio(driverConfig *config.DriverConfig) *minio.Client {
	endPoint := fmt.Sprintf("%s:%s", driverConfig.Minio.Host, driverConfig.Minio.Port)
	minioClient, err := minio.New(endPoint, &minio.Options{
		Creds:  credentials.NewStaticV4(driverConfig.Minio.Username, driverConfig.Minio.Password, ""),
		Secure: driverConfig.Minio.UseSSL,
	})
	if err != nil {
		log.Fatalf("Failed to initialize Minio Client: %s", err.Error())
	}

	log.Println("Successfully connected to minio")
	return minioClient
}
