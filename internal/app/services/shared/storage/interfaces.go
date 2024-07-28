package storage

import (
	"context"
	"mime/multipart"
)

type Storage interface {
	UploadFile(ctx context.Context, file multipart.File, fileHeader *multipart.FileHeader) (string, error)
}
