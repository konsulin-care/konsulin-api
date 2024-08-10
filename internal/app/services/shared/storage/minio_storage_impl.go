package storage

import (
	"context"
	"fmt"
	"io"
	"konsulin-service/internal/pkg/exceptions"
	"mime"
	"mime/multipart"
	"strings"

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

func (m *minioStorage) UploadBase64Image(ctx context.Context, encodedImageData []byte, bucketName, fileName, fileExtension string) (string, error) {
	contentType := mime.TypeByExtension(fileExtension)
	if contentType == "" {
		errContentType := fmt.Errorf("unknown content type for extension %s" + fileExtension)
		return "", exceptions.ErrMinioCreateObject(errContentType, bucketName)
	}

	_, err := m.MinioClient.PutObject(
		ctx,
		bucketName,
		fileName,
		strings.NewReader(string(encodedImageData)),
		int64(len(encodedImageData)),
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		return "", exceptions.ErrMinioCreateObject(err, bucketName)
	}

	return fileName, nil
}
