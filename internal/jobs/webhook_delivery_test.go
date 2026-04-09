package jobs

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsPrivateIP_Loopback verifies loopback addresses are classified as private.
func TestIsPrivateIP_Loopback(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		{"::1", true},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			assert.Equal(t, tt.private, isPrivateIP(ip),
				"expected isPrivateIP(%s) = %v", tt.ip, tt.private)
		})
	}
}

// TestIsPrivateIP_PrivateRanges verifies RFC 1918 (IPv4), RFC 4193 (IPv6 ULA),
// and RFC 4291 (IPv6 link-local) private ranges are blocked.
func TestIsPrivateIP_PrivateRanges(t *testing.T) {
	privateIPs := []string{
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.255.255",
		"169.254.0.1", // link-local (RFC 3927)
		"fc00::1",     // IPv6 ULA (RFC 4193)
		"fd00::1",     // IPv6 ULA (RFC 4193)
		"fe80::1",     // IPv6 link-local (RFC 4291)
	}
	for _, ip := range privateIPs {
		t.Run(ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			assert.True(t, isPrivateIP(parsed),
				"private address %s must be blocked", ip)
		})
	}
}

// TestIsPrivateIP_PublicIP verifies public addresses are not blocked.
func TestIsPrivateIP_PublicIP(t *testing.T) {
	publicIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"104.21.0.1",
		"2606:4700:4700::1111", // Cloudflare IPv6 DNS
	}
	for _, ip := range publicIPs {
		t.Run(ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			assert.False(t, isPrivateIP(parsed),
				"public IP %s must NOT be blocked", ip)
		})
	}
}

// TestReservedHeaders verifies that reserved header names are correctly identified.
func TestReservedHeaders(t *testing.T) {
	tests := []struct {
		header   string
		reserved bool
	}{
		{"content-type", true},
		{"x-raven-signature", true},
		{"x-raven-event", true},
		{"x-custom-header", false},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			_, isReserved := reservedHeaders[tt.header]
			assert.Equal(t, tt.reserved, isReserved,
				"expected reservedHeaders[%q] = %v", tt.header, tt.reserved)
		})
	}
}

// TestPrivateIPNets_InitialisedCorrectly verifies the init() function populated
// the CIDR ranges correctly.
func TestPrivateIPNets_InitialisedCorrectly(t *testing.T) {
	// At least the standard 7 CIDR blocks must be present (IPv4 + IPv6 ranges).
	assert.GreaterOrEqual(t, len(privateIPNets), 7,
		"privateIPNets must include all standard private/reserved CIDRs")
}
