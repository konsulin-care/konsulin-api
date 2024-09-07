package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func GenerateSessionJWT(sessionID, secret string, jwtExpiryTime int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"session_id": sessionID,
		"exp":        time.Now().Add(time.Duration(jwtExpiryTime) * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func GenerateResetPasswordJWT(uuid, secret string, jwtExpiryTime int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uuid": uuid,
		"exp":  time.Now().Add(time.Duration(jwtExpiryTime) * time.Minute).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func GenerateOTP(otpLength int) (string, error) {
	const otpDigits = "0123456789"
	max := big.NewInt(int64(len(otpDigits)))

	otp := make([]byte, otpLength)
	for i := range otp {
		num, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		otp[i] = otpDigits[num.Int64()]
	}

	return string(otp), nil
}

func GenerateFileName(prefix, username, fileExtension string) string {
	timestamp := time.Now().Format("20060102_150405.000000000")
	return fmt.Sprintf("%s_%s_%s%s", prefix, username, timestamp, fileExtension)
}
