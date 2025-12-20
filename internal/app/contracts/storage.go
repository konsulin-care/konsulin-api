package contracts

import (
	"context"
	"io"
	"mime/multipart"
	"time"
)

type Storage interface {
	UploadFile(ctx context.Context, file io.Reader, fileHeader *multipart.FileHeader, bucketName string) (string, error)
	GetObjectUrlWithExpiryTime(ctx context.Context, bucketName, objectName string, expiryTime time.Duration) (string, error)
	UploadBase64Image(ctx context.Context, encodedImage []byte, bucketName, fileName, fileExtension string) (string, error)
}
