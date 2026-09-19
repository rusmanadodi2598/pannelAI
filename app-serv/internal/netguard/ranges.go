// Package netguard validates an outbound destination before the gateway
// connects to it (OWASP A01 SSRF, docs/RULLES/OWASP.md).
//
// @file      internal/netguard/ranges.go
// @for       The address-range tables and predicates the guard's two tiers use.
// @uses      net/netip.
// @reason    The tiers are data, not control flow, and AGENTS.md §1.1 caps a
//
//	file at 250 lines: keeping the ranges here means a new never-a-host
//	range is a one-line addition to a table a reviewer can read whole,
//	rather than another branch inside CheckIP.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package netguard

import "net/netip"

// hardDeniedPrefixes are ranges that are never a proxy host, whatever the
// operator allowlists. CGNAT is here rather than in the opt-in tier because it
// is carrier address space, not private LAN space — and it is where the Alibaba
// metadata address (100.100.100.200) lives.
var hardDeniedPrefixes = []struct {
	prefix netip.Prefix
	reason string
}{
	{netip.MustParsePrefix("0.0.0.0/8"), "the this-network range"},
	{netip.MustParsePrefix("100.64.0.0/10"), "a carrier-grade NAT address"},
	{netip.MustParsePrefix("192.0.0.0/24"), "the IETF protocol assignment range"},
	// The broadcast address is listed before the reserved range it sits in, so
	// the reason names the address rather than the enclosing block.
	{netip.MustParsePrefix("255.255.255.255/32"), "the broadcast address"},
	{netip.MustParsePrefix("240.0.0.0/4"), "the reserved range"},
}

// neverAHost reports why an address can never be a destination, or "" when it
// could be one.
func neverAHost(ip netip.Addr) string {
	switch {
	case ip.IsUnspecified():
		return "the unspecified address"
	case ip.IsLinkLocalUnicast():
		return "a link-local address"
	case ip.IsLinkLocalMulticast(), ip.IsInterfaceLocalMulticast(), ip.IsMulticast():
		return "a multicast address"
	}
	for _, entry := range hardDeniedPrefixes {
		if entry.prefix.Contains(ip) {
			return entry.reason
		}
	}
	return ""
}

// needsAllowlist reports why an address is refused unless the operator named
// it, or "" when it is public.
func needsAllowlist(ip netip.Addr) string {
	switch {
	case ip.IsLoopback():
		return "a loopback address"
	case ip.IsPrivate():
		// netip covers both RFC 1918 and the IPv6 unique-local range here.
		return "a private address"
	}
	return ""
}

// embeddedIPv4 extracts the IPv4 address a transition address carries: the
// NAT64 well-known prefix, the 6to4 range, and the Teredo range. Without this,
// 64:ff9b::7f00:1 would reach 127.0.0.1 while passing every IPv4 rule.
func embeddedIPv4(ip netip.Addr) (netip.Addr, bool) {
	if !ip.Is6() {
		return netip.Addr{}, false
	}
	raw := ip.As16()
	switch {
	case netip.MustParsePrefix("64:ff9b::/96").Contains(ip):
		return netip.AddrFrom4([4]byte{raw[12], raw[13], raw[14], raw[15]}), true
	case netip.MustParsePrefix("2002::/16").Contains(ip):
		return netip.AddrFrom4([4]byte{raw[2], raw[3], raw[4], raw[5]}), true
	case netip.MustParsePrefix("2001::/32").Contains(ip):
		// Teredo stores the client's IPv4 address obfuscated: each bit is
		// inverted, so the address is recovered by XOR with 0xffffffff.
		return netip.AddrFrom4([4]byte{^raw[12], ^raw[13], ^raw[14], ^raw[15]}), true
	default:
		return netip.Addr{}, false
	}
}
