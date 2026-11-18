package main

import (
	"bytes"
	"errors"
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

func TestVerifyUsageErrors(t *testing.T) {
	var out bytes.Buffer
	p := writeEnv(t, "APP_ENV=dev\n")
	err := verify([]string{p, p}, &out, &out)
	if !errors.Is(err, errUsage) {
		t.Fatalf("want usage error for two FILE args, got %v", err)
	}
	err = verify([]string{"--standalone", "--federation"}, &out, &out)
	if !errors.Is(err, errUsage) {
		t.Fatalf("want usage error for exclusive modes, got %v", err)
	}
}

func TestVerifyPrintsFindingsAndCountsFails(t *testing.T) {
	var out bytes.Buffer
	err := verify([]string{writeEnv(t, "APP_ENV=dev\n")}, &out, &out)
	if err == nil {
		t.Fatal("want an error naming the FAIL count")
	}
	if !strings.Contains(err.Error(), "2 FAIL finding(s)") {
		t.Fatalf("want 2 FAILs, got %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if !strings.HasPrefix(lines[0], "mode=standalone") {
		t.Fatalf("want mode line first, got %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "FAIL ") && !strings.HasPrefix(line, "WARN ") {
			t.Fatalf("finding line must start with a level: %q", line)
		}
	}
	joined := out.String()
	for _, want := range []string{"FAIL SUPERADMIN_API_KEY: ", "FAIL APP_SUPERTOKEN_KONSULIN_DASHBOARD_ADMIN_EMAIL: "} {
		if !strings.Contains(joined, want) {
			t.Fatalf("want finding line %q in %q", want, joined)
		}
	}
}

func TestVerifySkipsOnlineCheckOnKeyFail(t *testing.T) {
	var out bytes.Buffer
	body := "APP_ENV=dev\nSUPERADMIN_API_KEY=k\nAPP_SUPERTOKEN_KONSULIN_DASHBOARD_ADMIN_EMAIL=a@b.co\nAPP_GATEWAY_URL=http://127.0.0.1:1\nAPP_PARTY_ID=p1\nAPP_AUTOMATION_PARTY_ID=a\n"
	err := verify([]string{"--federation", writeEnv(t, body)}, &out, &out)
	if err == nil {
		t.Fatal("want an error naming the FAIL count")
	}
	if strings.Contains(out.String(), "online check") {
		t.Fatalf("online check must be skipped when a party key FAILs: %q", out.String())
	}
	if !strings.Contains(out.String(), "FAIL APP_PARTY_PRIVATE_KEY: ") {
		t.Fatalf("want signing key FAIL: %q", out.String())
	}
	if !strings.Contains(err.Error(), "2 FAIL finding(s)") {
		t.Fatalf("want only the two key FAILs, got %v", err)
	}
}
