package middlewares

import (
	"net/http/httptest"
	"testing"
)

// TestRemoteAddrKey locks the rate-limit keying contract: a byte-for-byte
// clone of httprate's deprecated KeyByIP (SplitHostPort on RemoteAddr with
// raw fallback, then httprate.CanonicalizeIP — IPv4 unchanged, IPv6 /64).
func TestRemoteAddrKey(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"ipv4 strips port", "192.168.1.10:53000", "192.168.1.10"},
		{"ipv6 masked to /64", "[2001:db8:abcd:1234:5678:9abc:def0:1]:53000", "2001:db8:abcd:1234::"},
		{"no port falls back to raw", "plain-host", "plain-host"},
		{"empty addr stays empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://example.com/", nil)
			r.RemoteAddr = tt.remoteAddr
			got, err := remoteAddrKey(r)
			if err != nil {
				t.Fatalf("remoteAddrKey() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("remoteAddrKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
