//go:build ebpf

// Package helpers provides shared test utilities for eBPF integration tests.
package helpers

// BuildEthernetIPv4Packet constructs a minimal Ethernet + IPv4 packet for XDP
// program testing. srcIP and dstIP must be 4-byte IPv4 addresses.
func BuildEthernetIPv4Packet(srcIP, dstIP [4]byte) []byte {
	// Ethernet header: 14 bytes
	// IPv4 header:     20 bytes (no options)
	// Total:           34 bytes minimum
	pkt := make([]byte, 34)

	// Ethernet: dst MAC (broadcast), src MAC (zeroes), EtherType = IPv4 (0x0800)
	pkt[12] = 0x08
	pkt[13] = 0x00

	// IPv4 header
	pkt[14] = 0x45 // Version=4, IHL=5 (20 bytes)
	pkt[15] = 0x00 // DSCP/ECN
	// Total length = 20 (header only, no payload)
	pkt[16] = 0x00
	pkt[17] = 0x14

	pkt[22] = 0x40 // TTL = 64
	pkt[23] = 0x06 // Protocol = TCP

	// Source IP
	copy(pkt[26:30], srcIP[:])
	// Destination IP
	copy(pkt[30:34], dstIP[:])

	return pkt
}
