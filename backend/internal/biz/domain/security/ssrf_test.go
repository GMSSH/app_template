package security_test

import (
	"context"
	"net"
	"testing"
	"time"

	"gmssh-cli-plugin/internal/biz/domain/security"
)

func TestNormalizeDomain(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"example.com", "example.com", false},
		{"https://example.com/path", "example.com", false},
		{"http://SUB.EXAMPLE.COM:8080/", "sub.example.com", false},
		{"", "", true},
		{"-invalid.com", "", true},
		{"invalid-.com", "", true},
	}

	for _, c := range cases {
		out, err := security.NormalizeDomain(c.input)
		if (err != nil) != c.wantErr {
			t.Errorf("input %s: got err %v, wantErr %v", c.input, err, c.wantErr)
		}
		if !c.wantErr && out != c.expected {
			t.Errorf("input %s: got %s, expected %s", c.input, out, c.expected)
		}
	}
}

func TestSSRFProtection(t *testing.T) {
	privateIPs := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
		"::1",
		"fe80::1",
	}

	for _, ipStr := range privateIPs {
		ip := net.ParseIP(ipStr)
		if security.IsPublicIP(ip) {
			t.Errorf("IP %s should be blocked by SSRF check, but was allowed", ipStr)
		}
	}

	publicIPs := []string{
		"93.184.216.34",
		"1.1.1.1",
		"8.8.8.8",
		"223.5.5.5",
	}

	for _, ipStr := range publicIPs {
		ip := net.ParseIP(ipStr)
		if !security.IsPublicIP(ip) {
			t.Errorf("Public IP %s was wrongly blocked by SSRF check", ipStr)
		}
	}
}

func TestSafeDialSSRFBlocking(t *testing.T) {
	dialer := security.SafeDialContext(2 * time.Second)
	_, err := dialer(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil {
		t.Errorf("expected error connecting to 127.0.0.1, got nil")
	}
}
