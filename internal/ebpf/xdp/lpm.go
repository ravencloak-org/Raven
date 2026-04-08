package xdp

import (
	"encoding/binary"
	"fmt"
	"net"
)

// LPMKey mirrors the C struct bpf_lpm_trie_key + 4-byte IPv4 data.
type LPMKey struct {
	Prefixlen uint32
	Addr      [4]byte
}

// parseLPMKey parses a CIDR string into an LPMKey for the BPF LPM trie.
func parseLPMKey(cidr string) (LPMKey, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return LPMKey{}, fmt.Errorf("xdp: invalid CIDR %q: %w", cidr, err)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return LPMKey{}, fmt.Errorf("xdp: only IPv4 CIDRs supported, got %q", cidr)
	}
	ones, _ := ipNet.Mask.Size()
	var addr [4]byte
	binary.BigEndian.PutUint32(addr[:], binary.BigEndian.Uint32(ip4))
	return LPMKey{Prefixlen: uint32(ones), Addr: addr}, nil
}
