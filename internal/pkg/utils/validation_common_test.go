package utils

import (
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeBase64Image(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		valid       bool
	}{
		{"png", "data:image/png;base64,aGVsbG8=", true},
		{"jpeg", "data:image/jpeg;base64,aGVsbG8=", true},
		{"media type parameter", "data:image/png;charset=utf-8;base64,aGVsbG8=", true},
		{"no separator", "data:image/png;base64", false},
		{"invalid base64", "data:image/png;base64,!", false},
		{"empty header", ",aGVsbG8=", false},
		{"missing semicolon", "data:image/png,aGVsbG8=", false},
		{"not a data URI", "https:image/png;base64,aGVsbG8=", false},
		{"no content type", "data:;base64,aGVsbG8=", false},
		{"wrong encoding", "data:image/png;utf8,aGVsbG8=", false},
		{"unknown media type", "data:image/not-a-real-type;base64,aGVsbG8=", false},
		{"invalid media type", "data:invalid type;base64,aGVsbG8=", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data []byte
			var ext string
			var err error
			require.NotPanics(t, func() { data, ext, err = DecodeBase64Image(tc.input) })
			if tc.valid {
				require.NoError(t, err)
				require.Equal(t, []byte("hello"), data)
				if strings.Contains(tc.input, "png") {
					require.Equal(t, ".png", ext)
				} else {
					require.Equal(t, "image/jpeg", mime.TypeByExtension(ext))
				}
			} else {
				require.Error(t, err)
				require.Nil(t, data)
				require.Empty(t, ext)
			}
		})
	}
}

func TestImageValidation(t *testing.T) {
	require.NoError(t, ValidateImage(nil, 10))
	for _, ext := range []string{".jpg", ".jpeg", ".png"} {
		require.NoError(t, ValidateImage(&multipart.FileHeader{Filename: "photo" + ext, Size: 10}, 10))
	}
	require.Error(t, ValidateImage(&multipart.FileHeader{Filename: "photo.png", Size: 11}, 10))
	require.Error(t, ValidateImage(&multipart.FileHeader{Filename: "photo.txt", Size: 10}, 10))
	require.NoError(t, ValidateImageFormat(".png", []string{".jpg", ".png"}))
	require.Error(t, ValidateImageFormat(".exe", []string{".jpg", ".png"}))
	require.Error(t, ValidateImageFormat(".png", nil))
	require.NoError(t, ValidateImageSize(make([]byte, 1024*1024), 1))
	require.Error(t, ValidateImageSize(make([]byte, 1024*1024+1), 1))
	require.NoError(t, ValidateUrlParamID("patient-1"))
	require.Error(t, ValidateUrlParamID(""))
}

func TestPhoneNormalizationAndValidation(t *testing.T) {
	require.Equal(t, "6281234567890", NormalizePhoneDigits(" +62 (812) 3456-7890 "))
	for _, tc := range []struct {
		input string
		valid bool
	}{
		{"", false}, {"abc", false}, {"081234567890", false}, {"123456789", false}, {"1234567890123456", false}, {"1234567890", true}, {"123456789012345", true}, {" 6281234567890 ", true}, {"+6281234567890", false},
	} {
		err := ValidateInternationalPhoneDigits(tc.input)
		if tc.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
	require.Empty(t, FormatE164WithPlus(" "))
	require.Equal(t, "+6281234567890", FormatE164WithPlus(" 6281234567890 "))
	require.Equal(t, "+6281234567890", FormatE164WithPlus(" +6281234567890 "))
}
