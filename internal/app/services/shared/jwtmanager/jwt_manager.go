package jwtmanager

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/pkg/constvars"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
)

const (
	algES256 = "ES256"
	algRS256 = "RS256"
)

// JWTManager handles JWT creation and verification for webhook calls.
type JWTManager struct {
	log     *zap.Logger
	alg     string
	ttl     time.Duration
	ecPriv  *ecdsa.PrivateKey
	rsaPriv *rsa.PrivateKey
}

// CreateTokenInput defines input parameters for token creation.
type CreateTokenInput struct {
	Subject string
	// Payload is reserved for future custom claims. Currently unused by design.
	Payload map[string]interface{}
}

// CreateTokenOutput contains the signed token string.
type CreateTokenOutput struct {
	Token string
}

// VerifyTokenInput defines parameters for token verification.
type VerifyTokenInput struct {
	Token string
}

// VerifyTokenOutput contains verification result and decoded parts.
type VerifyTokenOutput struct {
	Valid  bool
	Header map[string]interface{}
	Claims map[string]interface{}
}

// NewJWTManager constructs a JWTManager using InternalConfig.Webhook and a logger.
// - Algorithm: ES256 (default) or RS256 via InternalConfig.Webhook.JWTAlg
// - Private key: PEM string from InternalConfig.Webhook.JWTHookKey
// - TTL: fixed 5 minutes per requirements
func NewJWTManager(cfg *config.InternalConfig, log *zap.Logger) (*JWTManager, error) {
	alg := strings.ToUpper(strings.TrimSpace(cfg.Webhook.JWTAlg))
	if alg == "" {
		alg = algES256
	}

	pemKey := strings.TrimSpace(cfg.Webhook.JWTHookKey)
	if pemKey == "" {
		return nil, fmt.Errorf("JWT_HOOK_KEY is empty")
	}

	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM for JWT_HOOK_KEY")
	}

	jm := &JWTManager{
		log: log,
		alg: alg,
		ttl: 5 * time.Minute, // fixed by requirement
	}

	switch alg {
	case algES256:
		// Expect EC private key
		ecKey, err := parseECPrivateKey(block)
		if err != nil {
			return nil, err
		}
		jm.ecPriv = ecKey
	case algRS256:
		rsaKey, err := parseRSAPrivateKey(block)
		if err != nil {
			return nil, err
		}
		jm.rsaPriv = rsaKey
	default:
		return nil, fmt.Errorf("unsupported JWT algorithm: %s", alg)
	}

	return jm, nil
}

// CreateToken generates a signed JWT with standard claims and the given subject.
// It sets iat, nbf to now and exp to now + 5 minutes.
func (j *JWTManager) CreateToken(ctx context.Context, in *CreateTokenInput) (*CreateTokenOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	j.log.Info("JWTManager.CreateToken called", zap.String(constvars.LoggingRequestIDKey, requestID))

	if in == nil || strings.TrimSpace(in.Subject) == "" {
		return nil, fmt.Errorf("subject is required")
	}

	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub": in.Subject,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": now.Add(j.ttl).Unix(),
	}

	var token *jwt.Token
	switch j.alg {
	case algES256:
		token = jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		signed, err := token.SignedString(j.ecPriv)
		if err != nil {
			return nil, err
		}
		return &CreateTokenOutput{Token: signed}, nil
	case algRS256:
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		signed, err := token.SignedString(j.rsaPriv)
		if err != nil {
			return nil, err
		}
		return &CreateTokenOutput{Token: signed}, nil
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", j.alg)
	}
}

// VerifyToken validates a token's signature and expiry and returns decoded header and claims.
func (j *JWTManager) VerifyToken(ctx context.Context, in *VerifyTokenInput) (*VerifyTokenOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	j.log.Info("JWTManager.VerifyToken called", zap.String(constvars.LoggingRequestIDKey, requestID))

	if in == nil || strings.TrimSpace(in.Token) == "" {
		return &VerifyTokenOutput{Valid: false}, fmt.Errorf("token is required")
	}

	keyFunc := func(t *jwt.Token) (interface{}, error) {
		// enforce expected alg
		if t.Method.Alg() != j.alg {
			return nil, fmt.Errorf("unexpected signing method: %s", t.Header["alg"])
		}
		switch j.alg {
		case algES256:
			if j.ecPriv == nil {
				return nil, errors.New("ec private key not loaded")
			}
			pub := j.ecPriv.Public().(*ecdsa.PublicKey)
			return pub, nil
		case algRS256:
			if j.rsaPriv == nil {
				return nil, errors.New("rsa private key not loaded")
			}
			pub := j.rsaPriv.Public().(*rsa.PublicKey)
			return pub, nil
		default:
			return nil, fmt.Errorf("unsupported algorithm: %s", j.alg)
		}
	}

	parsed, err := jwt.Parse(in.Token, keyFunc)
	if err != nil {
		return &VerifyTokenOutput{Valid: false}, nil
	}

	// Extract header
	header := make(map[string]interface{}, len(parsed.Header))
	for k, v := range parsed.Header {
		header[k] = v
	}

	// Extract claims
	claims := make(map[string]interface{})
	if c, ok := parsed.Claims.(jwt.MapClaims); ok {
		for k, v := range c {
			claims[k] = v
		}
	}

	// Ensure expiry/nbf are valid
	if !parsed.Valid {
		return &VerifyTokenOutput{Valid: false, Header: header, Claims: claims}, nil
	}

	return &VerifyTokenOutput{Valid: true, Header: header, Claims: claims}, nil
}

func parseECPrivateKey(block *pem.Block) (*ecdsa.PrivateKey, error) {
	if block.Type == "EC PRIVATE KEY" {
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse EC private key: %w", err)
		}
		return key, nil
	}
	if block.Type == "PRIVATE KEY" { // PKCS#8 which may wrap EC
		keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		if ec, ok := keyAny.(*ecdsa.PrivateKey); ok {
			return ec, nil
		}
		return nil, fmt.Errorf("PKCS8 key is not ECDSA")
	}
	return nil, fmt.Errorf("unsupported EC PEM type: %s", block.Type)
}

func parseRSAPrivateKey(block *pem.Block) (*rsa.PrivateKey, error) {
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 private key: %w", err)
		}
		return key, nil
	case "PRIVATE KEY": // PKCS#8
		keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		if rsaKey, ok := keyAny.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("PKCS8 key is not RSA")
	default:
		return nil, fmt.Errorf("unsupported RSA PEM type: %s", block.Type)
	}
}
