package storage

import (
	"context"
	"io"
	"konsulin-service/internal/pkg/exceptions"
	"mime/multipart"

	"github.com/minio/minio-go/v7"
)

type minioStorage struct {
	MinioClient *minio.Client
}

func NewMinioStorage(minioClient *minio.Client) Storage {
	return &minioStorage{
		MinioClient: minioClient,
	}
}

func (m *minioStorage) UploadFile(ctx context.Context, file io.Reader, fileHeader *multipart.FileHeader, bucketName string) (string, error) {
	fileName := fileHeader.Filename
	_, err := m.MinioClient.PutObject(ctx, bucketName, fileName, file, fileHeader.Size, minio.PutObjectOptions{
		ContentType: fileHeader.Header.Get("Content-Type"),
	})
	if err != nil {
		return "", exceptions.ErrMinioCreateObject(err, bucketName)
	}

	return fileName, nil
}
