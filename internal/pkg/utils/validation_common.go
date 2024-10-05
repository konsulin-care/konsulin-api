package utils

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
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

func DecodeBase64Image(encodedImage string) ([]byte, string, error) {
	parts := strings.SplitN(encodedImage, ",", 2)
	if len(parts) != 2 {
		return nil, "", errors.New("invalid base64 image")
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", err
	}

	contentType := parts[0][5:strings.Index(parts[0], ";")]
	ext, err := mime.ExtensionsByType(contentType)
	if err != nil || len(ext) == 0 {
		return nil, "", errors.New("invalid image type")
	}

	return data, ext[0], nil
}

func ValidateImageFormat(ext string, allowedFormats []string) error {
	for _, format := range allowedFormats {
		if ext == format {
			return nil
		}
	}
	return fmt.Errorf("invalid image format. Allowed formats are: %s", strings.Join(allowedFormats, ", "))
}

func ValidateImageSize(data []byte, maxSize int) error {
	if len(data) > maxSize*1024*1024 {
		return fmt.Errorf("image exceeds maximum allowed size of %dMB", maxSize)
	}
	return nil
}
