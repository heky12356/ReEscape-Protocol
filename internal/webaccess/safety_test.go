package webaccess

import (
	"context"
	"fmt"
	"net"
	"testing"
)

type staticResolver map[string][]net.IPAddr

func (r staticResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if ips, ok := r[host]; ok {
		return ips, nil
	}
	return nil, fmt.Errorf("host not found: %s", host)
}

func TestValidateOutboundURLRejectsUnsafeTargets(t *testing.T) {
	resolver := staticResolver{
		"example.com": {
			{IP: net.ParseIP("93.184.216.34")},
		},
		"internal.example": {
			{IP: net.ParseIP("10.0.0.5")},
		},
	}

	tests := []string{
		"http://localhost",
		"http://127.0.0.1",
		"http://0.0.0.0",
		"http://10.1.2.3",
		"http://172.16.1.1",
		"http://192.168.1.1",
		"http://169.254.169.254",
		"http://[::1]",
		"file:///etc/passwd",
		"ftp://example.com/file",
		"http:///missing-host",
		"https://example.com:8443",
		"https://internal.example",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := validateOutboundURL(context.Background(), raw, resolver); err == nil {
				t.Fatalf("expected %q to be rejected", raw)
			}
		})
	}
}

func TestValidateOutboundURLAllowsPublicHTTPSTarget(t *testing.T) {
	resolver := staticResolver{
		"example.com": {
			{IP: net.ParseIP("93.184.216.34")},
		},
	}

	parsed, err := validateOutboundURL(context.Background(), "https://example.com/path", resolver)
	if err != nil {
		t.Fatalf("expected public target to be allowed: %v", err)
	}
	if got := parsed.String(); got != "https://example.com/path" {
		t.Fatalf("unexpected parsed url: %s", got)
	}
}
