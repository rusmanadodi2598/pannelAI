// Package clientip answers the one question the rate limiters ask: which
// client address is this request really from.
//
// @file      internal/clientip/clientip.go
// @for       The trusted-proxy rule for reading X-Forwarded-For, shared by the
//
//	gateway rate limiter and the login limiter.
//
// @uses      errors, fmt, net, strings.
// @reason    R20 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: the limiter bucketed
//
//	every client behind a reverse proxy into the proxy's one address.
//	Reading the forwarded header is the fix, but the header is
//	forgeable, so it is only read past a peer the operator explicitly
//	named as a proxy. One package owns the rule because two limiters
//	ask the same question and a rule that lives in two files is a rule
//	that decays.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package clientip

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// ParseTrusted compiles the operator's TRUSTED_PROXY_CIDRS list into the nets
// whose forwarded chains may be read. An empty list is the default and means
// "no proxy is trusted", which keeps every deployment on the peer address it
// always used. A malformed entry fails the boot rather than installing a trust
// set nobody can reason about.
func ParseTrusted(cidrs []string) ([]*net.IPNet, error) {
	trusted := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		parsed, err := parseEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("clientip: %q is not a CIDR or address: %w", raw, err)
		}
		trusted = append(trusted, parsed)
	}
	return trusted, nil
}

// parseEntry turns one operator entry into a net. A bare address is trusted as
// exactly one address, which means the host mask of its own family. Suffixing a
// fixed "/32" is not a cosmetic choice: net.ParseCIDR accepts "2001:db8::1/32"
// and hands back roughly 2^96 addresses, so naming one IPv6 proxy would have
// made the forwarded header, and with it the limiter bucket, forgeable from
// anywhere in that block.
func parseEntry(entry string) (*net.IPNet, error) {
	if strings.Contains(entry, "/") {
		_, parsed, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	}

	ip := net.ParseIP(entry)
	if ip == nil {
		return nil, errors.New("no address to read")
	}
	bits := 128
	if v4 := ip.To4(); v4 != nil {
		ip, bits = v4, 32
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}, nil
}

// Address answers which client address the request is bucketed under. It takes
// the request's own address and its forwarded chain as plain values, so the
// package carries no HTTP types and a caller that only has a peer and a header
// string can ask the same question. A peer that is not a trusted proxy is
// answered with itself and the chain is never read. A trusted proxy's chain is
// walked right to left, and the rightmost hop that is not itself trusted is the
// client, the one address in the chain nobody downstream of a trusted proxy
// could have forged. A chain of nothing but trusted hops falls back to the
// peer, so the bucket always names an address the operator can hold
// accountable.
func Address(remoteAddr, forwardedFor string, trusted []*net.IPNet) string {
	peer := remoteHost(remoteAddr)
	ip := net.ParseIP(peer)
	if ip == nil || !trustedProxy(ip, trusted) {
		return peer
	}

	chain := strings.Split(forwardedFor, ",")
	for i := len(chain) - 1; i >= 0; i-- {
		hop := net.ParseIP(strings.TrimSpace(chain[i]))
		if hop == nil {
			break
		}
		if trustedProxy(hop, trusted) {
			continue
		}
		return hop.String()
	}
	return peer
}

// remoteHost strips the port from a peer address, falling back to the raw
// value when it carries no port, the shape the limiter always bucketed by.
func remoteHost(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}

// trustedProxy reports whether the address is one of the nets the operator
// named. An empty trust set refuses everything, which is what keeps the
// header unread when no proxy is configured.
func trustedProxy(ip net.IP, trusted []*net.IPNet) bool {
	for _, net_ := range trusted {
		if net_.Contains(ip) {
			return true
		}
	}
	return false
}
