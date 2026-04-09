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

// TestIsPrivateIP_RFC1918 verifies RFC 1918 private ranges are blocked.
func TestIsPrivateIP_RFC1918(t *testing.T) {
	privateIPs := []string{
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.255.255",
		"169.254.0.1", // link-local
	}
	for _, ip := range privateIPs {
		t.Run(ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			assert.True(t, isPrivateIP(parsed),
				"RFC 1918 address %s must be blocked", ip)
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
func TestReservedHeaders_ContentType(t *testing.T) {
	_, isReserved := reservedHeaders["content-type"]
	assert.True(t, isReserved, "content-type must be a reserved header")
}

func TestReservedHeaders_XRavenSignature(t *testing.T) {
	_, isReserved := reservedHeaders["x-raven-signature"]
	assert.True(t, isReserved, "x-raven-signature must be a reserved header")
}

func TestReservedHeaders_XRavenEvent(t *testing.T) {
	_, isReserved := reservedHeaders["x-raven-event"]
	assert.True(t, isReserved, "x-raven-event must be a reserved header")
}

func TestReservedHeaders_CustomHeader_NotReserved(t *testing.T) {
	_, isReserved := reservedHeaders["x-custom-header"]
	assert.False(t, isReserved, "custom headers must not be in reserved set")
}

// TestPrivateIPNets_InitialisedCorrectly verifies the init() function populated
// the CIDR ranges correctly.
func TestPrivateIPNets_InitialisedCorrectly(t *testing.T) {
	// At least the standard 7 CIDR blocks must be present (IPv4 + IPv6 ranges).
	assert.GreaterOrEqual(t, len(privateIPNets), 7,
		"privateIPNets must include all standard private/reserved CIDRs")
}
