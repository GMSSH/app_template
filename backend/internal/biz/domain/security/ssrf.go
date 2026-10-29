package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

var (
	ErrEmptyDomain   = errors.New("domain is empty")
	ErrInvalidDomain = errors.New("invalid domain format")
	ErrDomainTooLong = errors.New("domain exceeds maximum length of 253 characters")
	ErrSSRFBlocked   = errors.New("connection blocked: destination IP is private, loopback, or reserved")
)

// NormalizeDomain sanitizes and converts a domain to punycode ASCII.
func NormalizeDomain(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	if idx := strings.Index(raw, "/"); idx != -1 {
		raw = raw[:idx]
	}
	if idx := strings.Index(raw, ":"); idx != -1 {
		raw = raw[:idx]
	}
	raw = strings.TrimSuffix(raw, ".")
	if raw == "" {
		return "", ErrEmptyDomain
	}
	if len(raw) > 253 {
		return "", ErrDomainTooLong
	}

	ascii, err := idna.Lookup.ToASCII(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidDomain, err)
	}
	ascii = strings.ToLower(ascii)

	// Check domain labels
	labels := strings.Split(ascii, ".")
	if len(labels) < 2 && ascii != "localhost" {
		return "", fmt.Errorf("%w: domain must have at least one dot", ErrInvalidDomain)
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return "", fmt.Errorf("%w: label length must be 1-63", ErrInvalidDomain)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("%w: label cannot begin or end with hyphen", ErrInvalidDomain)
		}
	}

	return ascii, nil
}

// IsPublicIP verifies if an IP address is a safe, routable public IP.
func IsPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	netipAddr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	netipAddr = netipAddr.Unmap()

	if netipAddr.IsLoopback() ||
		netipAddr.IsPrivate() ||
		netipAddr.IsLinkLocalUnicast() ||
		netipAddr.IsLinkLocalMulticast() ||
		netipAddr.IsInterfaceLocalMulticast() ||
		netipAddr.IsMulticast() ||
		netipAddr.IsUnspecified() {
		return false
	}

	// Check for cloud metadata addresses (e.g. 169.254.169.254, 100.100.100.200)
	ipStr := netipAddr.String()
	if ipStr == "169.254.169.254" || ipStr == "100.100.100.200" {
		return false
	}

	return true
}

// SafeDialContext creates a dialer that strictly enforces public IP destinations.
func SafeDialContext(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
			port = ""
		}

		// Resolve host IPs
		resolver := net.DefaultResolver
		ips, err := resolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}

		var publicIPs []net.IP
		for _, ip := range ips {
			if IsPublicIP(ip) {
				publicIPs = append(publicIPs, ip)
			}
		}

		if len(publicIPs) == 0 {
			return nil, fmt.Errorf("%w (%s resolves to private/internal IPs only)", ErrSSRFBlocked, host)
		}

		// Dial the first valid public IP
		dialer := &net.Dialer{
			Timeout:   timeout,
			KeepAlive: 15 * time.Second,
		}

		target := net.JoinHostPort(publicIPs[0].String(), port)
		return dialer.DialContext(ctx, network, target)
	}
}
