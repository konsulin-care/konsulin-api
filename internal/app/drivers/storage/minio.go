package storage

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioClient struct {
	Client               *minio.Client
	BucketName           string
	MaxUploadSizeInBytes int64
}

func NewMinio(driverConfig *config.DriverConfig) *minio.Client {
	endPoint := fmt.Sprintf("%s:%s", driverConfig.Minio.Host, driverConfig.Minio.Port)
	minioClient, err := minio.New(endPoint, &minio.Options{
		Creds:  credentials.NewStaticV4(driverConfig.Minio.Username, driverConfig.Minio.Password, ""),
		Secure: driverConfig.Minio.UseSSL,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Minio: %s", err.Error())
	}
	log.Println("Successfully connected to minio")
	return minioClient

	// bucketName := os.Getenv("MINIO_BUCKET_NAME")
	// err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: ""})
	// if err != nil {
	// 	exists, errBucketExists := minioClient.BucketExists(context.Background(), bucketName)
	// 	if errBucketExists == nil && exists {
	// 		log.Printf("Bucket %s already exists\n", bucketName)
	// 	} else {
	// 		return nil, err
	// 	}
	// }

	// maxUploadSizeMB, err := strconv.ParseInt(os.Getenv("MINIO_PROFILE_PICTURE_UPLOAD_MAX_SIZE_IN_MB"), 10, 64)
	// if err != nil {
	// 	maxUploadSizeMB = 2 // Default to 2MB if the environment variable is not set
	// }
	// maxUploadSizeBytes := maxUploadSizeMB * 1024 * 1024

	// return &MinioClient{
	// 	Client:               minioClient,
	// 	BucketName:           bucketName,
	// 	MaxUploadSizeInBytes: maxUploadSizeBytes,
	// }, nil
}
