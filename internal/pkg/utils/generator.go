package utils

import (
	"crypto/rand"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"math/big"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// GenerateRequestID creates a unique transaction ID.
//
// The ID is generated using the current time in nanoseconds
// since the Unix epoch, ensuring high precision and uniqueness
// even in high-concurrency environments. The format of the ID
// is "<unix nano timestamp>".
func GenerateRequestID() string {
	transID := time.Now().UnixNano()
	return fmt.Sprintf("%s%d", constvars.REQUEST_ID_PREFIX, transID)
}

// signJWT signs the given claims with HS256 using the provided secret.
func signJWT(claims jwt.MapClaims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func GenerateSessionJWT(sessionID, secret string, jwtExpiryTime int) (string, error) {
	return signJWT(jwt.MapClaims{
		"session_id": sessionID,
		"exp":        time.Now().Add(time.Duration(jwtExpiryTime) * time.Hour).Unix(),
	}, secret)
}

func GenerateResetPasswordJWT(uuid, secret string, jwtExpiryTime int) (string, error) {
	return signJWT(jwt.MapClaims{
		"uuid": uuid,
		"exp":  time.Now().Add(time.Duration(jwtExpiryTime) * time.Minute).Unix(),
	}, secret)
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

func GenerateMagicLink(baseURL, preAuthSessionID, tenantID, linkCode string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	q := u.Query()
	q.Set("preAuthSessionId", preAuthSessionID)
	q.Set("tenantId", tenantID)
	u.RawQuery = q.Encode()

	u.Fragment = linkCode

	return u.String(), nil
}
