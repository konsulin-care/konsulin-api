package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnv(t *testing.T, body string) string {
	p := filepath.Join(t.TempDir(), "e.env")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerifyModeInference(t *testing.T) {
	t.Setenv("APP_GATEWAY_URL", "https://from-process.example")
	var out bytes.Buffer
	_ = verify([]string{writeEnv(t, "APP_ENV=dev\n")}, &out, &out)
	if !strings.Contains(out.String(), "mode=standalone (inferred)") {
		t.Fatalf("got %q", out.String())
	}
	out.Reset()
	_ = verify([]string{writeEnv(t, "APP_GATEWAY_URL=https://g.example\n")}, &out, &out)
	if !strings.Contains(out.String(), "mode=federation (inferred)") {
		t.Fatalf("got %q", out.String())
	}
	out.Reset()
	_ = verify([]string{"--standalone", writeEnv(t, "APP_GATEWAY_URL=https://g.example\n")}, &out, &out)
	if !strings.Contains(out.String(), "mode=standalone (explicit)") {
		t.Fatalf("got %q", out.String())
	}
	if err := verify([]string{"--standalone", "--federation"}, &out, &out); err == nil {
		t.Fatal("want usage error")
	}
}
