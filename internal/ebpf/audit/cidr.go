// Package audit - shared helpers (no build tag, compiles everywhere).
package audit

import (
	"fmt"
	"net"
)

// parseCIDRs parses a slice of CIDR strings into *net.IPNet values.
func parseCIDRs(cidrs []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		nets = append(nets, n)
	}
	return nets, nil
}
