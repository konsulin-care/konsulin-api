package jwtmanager

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"konsulin-service/internal/app/config"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testKeys holds the key material shared by the round-trip and key-format cases.
type testKeys struct {
	ec                       *ecdsa.PrivateKey
	rsa                      *rsa.PrivateKey
	ecDER, ecPKCS8, rsaPKCS8 []byte
}

func newTestKeys(t *testing.T) testKeys {
	t.Helper()
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ecDER, err := x509.MarshalECPrivateKey(ec)
	require.NoError(t, err)
	ecPKCS8, err := x509.MarshalPKCS8PrivateKey(ec)
	require.NoError(t, err)
	rsaPKCS8, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	require.NoError(t, err)
	return testKeys{ec: ec, rsa: rsaKey, ecDER: ecDER, ecPKCS8: ecPKCS8, rsaPKCS8: rsaPKCS8}
}

// webhookConfig builds a config whose hook key is der wrapped in a PEM block of pemType.
func webhookConfig(alg, pemType string, der []byte) *config.InternalConfig {
	cfg := &config.InternalConfig{}
	cfg.Webhook.JWTAlg = alg
	cfg.Webhook.JWTHookKey = string(pem.EncodeToMemory(&pem.Block{Type: pemType, Bytes: der}))
	return cfg
}

// roundTripCase carries the expected algorithm plus the method/key used to forge foreign tokens.
type roundTripCase struct {
	name, alg, pemType string
	der                []byte
	wantAlg            string
	method             jwt.SigningMethod
	signKey            interface{}
}

func TestJWTManagerRoundTripAndKeyFormats(t *testing.T) {
	keys := newTestKeys(t)
	for _, tc := range []roundTripCase{
		{"default EC", "", "EC PRIVATE KEY", keys.ecDER, algES256, jwt.SigningMethodES256, keys.ec},
		{"EC PKCS8", " es256 ", "PRIVATE KEY", keys.ecPKCS8, algES256, jwt.SigningMethodES256, keys.ec},
		{"RSA PKCS1", "RS256", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(keys.rsa), algRS256, jwt.SigningMethodRS256, keys.rsa},
		{"RSA PKCS8", "rs256", "PRIVATE KEY", keys.rsaPKCS8, algRS256, jwt.SigningMethodRS256, keys.rsa},
	} {
		t.Run(tc.name, func(t *testing.T) { runRoundTripCase(t, tc) })
	}
	for _, tc := range []struct {
		alg, pemType string
		der          []byte
	}{
		{algES256, "EC PRIVATE KEY", []byte("bad")},
		{algES256, "PRIVATE KEY", []byte("bad")},
		{algES256, "PRIVATE KEY", keys.rsaPKCS8},
		{algES256, "RSA PRIVATE KEY", nil},
		{algRS256, "RSA PRIVATE KEY", []byte("bad")},
		{algRS256, "PRIVATE KEY", []byte("bad")},
		{algRS256, "PRIVATE KEY", keys.ecPKCS8},
		{algRS256, "EC PRIVATE KEY", nil},
		{"HS256", "EC PRIVATE KEY", keys.ecDER},
	} {
		got, err := NewJWTManager(webhookConfig(tc.alg, tc.pemType, tc.der), zap.NewNop())
		require.Error(t, err)
		require.Nil(t, got)
	}
	for _, key := range []string{"", "not PEM"} {
		cfg := &config.InternalConfig{}
		cfg.Webhook.JWTHookKey = key
		_, err := NewJWTManager(cfg, zap.NewNop())
		require.Error(t, err)
	}
}

func runRoundTripCase(t *testing.T, tc roundTripCase) {
	manager, err := NewJWTManager(webhookConfig(tc.alg, tc.pemType, tc.der), zap.NewNop())
	require.NoError(t, err)
	require.Equal(t, tc.wantAlg, manager.alg)
	created, err := manager.CreateToken(context.Background(), &CreateTokenInput{Subject: "service", Payload: map[string]interface{}{"ignored": "value"}})
	require.NoError(t, err)
	verified, err := manager.VerifyToken(context.Background(), &VerifyTokenInput{Token: created.Token})
	require.NoError(t, err)
	require.True(t, verified.Valid)
	require.Equal(t, "service", verified.Claims["sub"])
	require.Equal(t, manager.alg, verified.Header["alg"])
	require.Equal(t, float64(300), verified.Claims["exp"].(float64)-verified.Claims["iat"].(float64))
	require.Equal(t, verified.Claims["iat"], verified.Claims["nbf"])
	require.NotContains(t, verified.Claims, "ignored")
	assertRejectsInvalidInput(t, manager)
	assertRejectsBadTokens(t, manager, created.Token, tc.method, tc.signKey)
}

// assertRejectsInvalidInput checks that missing subjects and tokens are errors, not just invalid results.
func assertRejectsInvalidInput(t *testing.T, manager *JWTManager) {
	t.Helper()
	for _, input := range []*CreateTokenInput{nil, {}, {Subject: " "}} {
		_, err := manager.CreateToken(context.Background(), input)
		require.Error(t, err)
	}
	for _, input := range []*VerifyTokenInput{nil, {}, {Token: " "}} {
		got, err := manager.VerifyToken(context.Background(), input)
		require.Error(t, err)
		require.False(t, got.Valid)
	}
}

// assertRejectsBadTokens checks that malformed, tampered, expired and not-yet-valid tokens verify as invalid without error.
func assertRejectsBadTokens(t *testing.T, manager *JWTManager, valid string, method jwt.SigningMethod, key interface{}) {
	t.Helper()
	tokens := []string{"invalid", valid + "tampered"}
	for _, claims := range []jwt.MapClaims{{"exp": time.Now().Add(-time.Minute).Unix()}, {"nbf": time.Now().Add(time.Hour).Unix()}} {
		token, err := jwt.NewWithClaims(method, claims).SignedString(key)
		require.NoError(t, err)
		tokens = append(tokens, token)
	}
	for _, token := range tokens {
		got, err := manager.VerifyToken(context.Background(), &VerifyTokenInput{Token: token})
		require.NoError(t, err)
		require.False(t, got.Valid)
	}
}

func TestJWTVerificationRejectsMissingKeysAndWrongAlgorithm(t *testing.T) {
	for _, alg := range []string{algES256, algRS256} {
		manager := &JWTManager{alg: alg, log: zap.NewNop()}
		var method jwt.SigningMethod = jwt.SigningMethodES256
		if alg == algRS256 {
			method = jwt.SigningMethodRS256
		}
		_, err := manager.keyFunc(jwt.New(method))
		require.Error(t, err)
		_, err = manager.keyFunc(jwt.New(jwt.SigningMethodHS256))
		require.ErrorContains(t, err, "unexpected signing method")
	}
	manager := &JWTManager{alg: "HS256", log: zap.NewNop()}
	_, err := manager.CreateToken(context.Background(), &CreateTokenInput{Subject: "service"})
	require.ErrorContains(t, err, "unsupported algorithm")
	_, err = manager.keyFunc(jwt.New(jwt.SigningMethodHS256))
	require.ErrorContains(t, err, "unsupported algorithm")
	ec, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	manager.alg, manager.ecPriv = algES256, ec
	_, err = manager.CreateToken(context.Background(), &CreateTokenInput{Subject: "service"})
	require.Error(t, err)
	manager.alg = algRS256
	manager.rsaPriv = &rsa.PrivateKey{PublicKey: rsa.PublicKey{N: big.NewInt(33), E: 3}, D: big.NewInt(7), Primes: []*big.Int{big.NewInt(3), big.NewInt(11)}}
	_, err = manager.CreateToken(context.Background(), &CreateTokenInput{Subject: "service"})
	require.Error(t, err)
}

func TestJWTDecodedMapsAreIndependent(t *testing.T) {
	token := &jwt.Token{Header: map[string]interface{}{"alg": "ES256"}, Claims: jwt.MapClaims{"sub": "service"}}
	header, claims := extractHeader(token), extractClaims(token)
	header["alg"], claims["sub"] = "changed", "changed"
	require.Equal(t, "ES256", token.Header["alg"])
	require.Equal(t, "service", token.Claims.(jwt.MapClaims)["sub"])
	token.Claims = jwt.RegisteredClaims{}
	require.Nil(t, extractClaims(token))
}
