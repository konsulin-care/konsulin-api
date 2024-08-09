package utils

import (
	"errors"
	"mime/multipart"
	"strings"

	"github.com/google/uuid"
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

func ValidateUrlParamID(param string) error {
	if param == "" {
		return errors.New("parameter is missing from url path")
	}

	_, err := uuid.Parse(param)
	if err != nil {
		return err
	}

	return nil
}
