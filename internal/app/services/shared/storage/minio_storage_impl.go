package storage

import (
	"context"
	"konsulin-service/internal/app/config"
	"mime/multipart"

	"github.com/minio/minio-go/v7"
)

type minioStorage struct {
	MinioClient    *minio.Client
	BucketName     string
	InternalConfig *config.InternalConfig
}

func NewMinioStorage(minioClient *minio.Client, bucketName string) Storage {
	return &minioStorage{
		MinioClient: minioClient,
		BucketName:  bucketName,
	}
}

func (m *minioStorage) UploadFile(ctx context.Context, file multipart.File, fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Size > m.InternalConfig.Minio.ProfilePictureMaxUploadSizeInMB {
		// return "", fmt.Errorf("file size exceeds the maximum limit of %d MB", m.MaxSize/(1024*1024))
		return "", nil
	}

	// contentType := fileHeader.Header.Get("Content-Type")
	// if !strings.HasPrefix(contentType, "image/") {
	// 	return "", fmt.Errorf("only image files are allowed")
	// }

	// fileName := fileHeader.Filename
	// _, err := m.Client.PutObject(ctx, m.BucketName, fileName, file, fileHeader.Size, minio.PutObjectOptions{ContentType: contentType})
	// if err != nil {
	// 	return "", err
	// }

	// fileURL := fmt.Sprintf("%s/%s/%s", os.Getenv("MINIO_URL"), m.BucketName, fileName)
	return "", nil
}
