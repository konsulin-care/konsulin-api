package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/drivers/gateway"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func onlineEnvFile(t *testing.T, url, extra string) (string, config.AppGateway) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	sk := base64.StdEncoding.EncodeToString(seed)
	ek := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	body := fmt.Sprintf("APP_ENV=test\nAPP_GATEWAY_URL=%s\nAPP_PARTY_ID=p1\nAPP_AUTOMATION_PARTY_ID=a\nSUPERADMIN_API_KEY=k\nAPP_SUPERTOKEN_KONSULIN_DASHBOARD_ADMIN_EMAIL=a@b.co\nAPP_PARTY_PRIVATE_KEY=%s\nAPP_PARTY_ENC_PRIVATE_KEY=%s\n%s", url, sk, ek, extra)
	p := filepath.Join(t.TempDir(), "e.env")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p, config.AppGateway{Enabled: true, URL: url, PartyID: "p1", PrivateKey: sk, EncPrivateKey: ek}
}

func jwksServer(t *testing.T, kid string, hits *int32) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"keys":[{"kid":%q,"kty":"OKP"}]}`, kid)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVerifyOnlineKidMismatchFails(t *testing.T) {
	var hits int32
	srv := jwksServer(t, "other", &hits)
	p, _ := onlineEnvFile(t, srv.URL, "")
	var out bytes.Buffer
	err := verify([]string{"--federation", p}, &out, &out)
	require.Error(t, err)
	assert.Contains(t, out.String(), "JWKS kid mismatch")
}

func TestVerifyOnlineEnrolledWarnsOnToken(t *testing.T) {
	var hits int32
	srv := jwksServer(t, "x", &hits)
	p, cfg := onlineEnvFile(t, srv.URL, "APP_PARTY_ENROLLMENT_TOKEN=tok\n")
	c, err := gateway.New(cfg, "test", nil)
	require.NoError(t, err)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = fmt.Fprintf(w, `{"keys":[{"kid":%q}]}`, c.Kid())
	})
	var out bytes.Buffer
	require.NoError(t, verify([]string{"--federation", p}, &out, &out), out.String())
	assert.Contains(t, out.String(), "WARN APP_PARTY_ENROLLMENT_TOKEN")
	assert.NotContains(t, out.String(), "tok\n")
}

func TestVerifyOfflineMakesNoRequest(t *testing.T) {
	var hits int32
	srv := jwksServer(t, "other", &hits)
	p, _ := onlineEnvFile(t, srv.URL, "")
	var out bytes.Buffer
	_ = verify([]string{"--federation", "--offline", p}, &out, &out)
	assert.Zero(t, atomic.LoadInt32(&hits))
}
