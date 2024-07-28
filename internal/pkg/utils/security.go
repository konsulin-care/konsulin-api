package utils

import (
	"crypto/rand"
	"konsulin-service/internal/pkg/exceptions"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateSessionJWT(sessionID, secret string, jwtExpiryTime int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"session_id": sessionID,
		"exp":        time.Now().Add(time.Duration(jwtExpiryTime) * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", exceptions.ErrTokenGenerate(err)
	}

	return tokenString, nil
}

func GenerateResetPasswordJWT(uuid, secret string, jwtExpiryTime int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Duration(jwtExpiryTime) * time.Minute).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", exceptions.ErrTokenGenerate(err)
	}

	return tokenString, nil
}

func ParseJWT(tokenString, secret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, exceptions.ErrTokenSigningMethod(nil)
		}
		return []byte(secret), nil
	})

	if err != nil {
		return "", exceptions.ErrTokenInvalid(err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sessionID, ok := claims["session_id"].(string); ok {
			return sessionID, nil
		}
	}

	return "", exceptions.ErrTokenInvalid(nil)
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
