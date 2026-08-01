package webaccess

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type defaultResolver struct{}

func (defaultResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

func ValidateOutboundURL(ctx context.Context, raw string) (*url.URL, error) {
	return validateOutboundURL(ctx, raw, defaultResolver{})
}

func validateOutboundURL(ctx context.Context, raw string, r resolver) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("url scheme must be http or https")
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return nil, fmt.Errorf("url host is required")
	}
	if isLocalhostName(host) {
		return nil, fmt.Errorf("localhost is not allowed")
	}
	if port := strings.TrimSpace(parsed.Port()); port != "" && port != "80" && port != "443" {
		return nil, fmt.Errorf("non-standard ports are not allowed")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeIP(ip) {
			return nil, fmt.Errorf("private or local ip is not allowed")
		}
		return parsed, nil
	}

	ips, err := r.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("resolve host returned no addresses")
	}
	for _, ipAddr := range ips {
		if isUnsafeIP(ipAddr.IP) {
			return nil, fmt.Errorf("private or local resolved ip is not allowed")
		}
	}

	return parsed, nil
}

func isLocalhostName(host string) bool {
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return normalized == "localhost"
}

func isUnsafeIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 169 && v4[1] == 254
	}
	return false
}
