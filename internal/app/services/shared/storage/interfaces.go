package storage

import (
	"context"
	"io"
	"mime/multipart"
)

type Storage interface {
	UploadFile(ctx context.Context, file io.Reader, fileHeader *multipart.FileHeader, bucketName string) (string, error)
}
