package utils

import (
	"errors"

	"github.com/golang-jwt/jwt/v4"
)

func ParseJWT(tokenString, secret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid token signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sessionID, ok := claims["session_id"].(string); ok {
			return sessionID, nil
		}
	}

	return "", errors.New("invalid token")
}
