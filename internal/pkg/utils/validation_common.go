package utils

import (
	"errors"
	"mime/multipart"
	"strings"
)

func ValidateImage(fileHeader *multipart.FileHeader, maxSizeInMegabytes int64) error {
	if fileHeader == nil {
		return nil
	}

	if fileHeader.Size > maxSizeInMegabytes {
		return errors.New("file size exceeds the maximum limit")
	}

	validExtensions := []string{".jpg", ".jpeg", ".png"}
	for _, ext := range validExtensions {
		if strings.HasSuffix(fileHeader.Filename, ext) {
			return nil
		}
	}
	return errors.New("invalid file format")
}
