package utils

import (
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
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

func GenerateJWT(sessionID, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"session_id": sessionID,
		"exp":        time.Now().Add(time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevAuthGenerateToken)
	}

	return tokenString, nil
}

func ParseJWT(tokenString, secret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, exceptions.WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevAuthSigningMethod)
		}
		return []byte(secret), nil
	})

	if err != nil {
		return "", exceptions.WrapWithError(err, constvars.StatusUnauthorized, constvars.ErrClientNotLoggedIn, constvars.ErrDevAuthTokenInvalid)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sessionID, ok := claims["session_id"].(string); ok {
			return sessionID, nil
		}
	}

	return "", exceptions.WrapWithoutError(constvars.StatusUnauthorized, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevAuthTokenInvalid)
}
