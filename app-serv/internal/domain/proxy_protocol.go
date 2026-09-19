// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/proxy_protocol.go
// @for       The proxy protocol set and the host/port value rules §7.11 stores.
// @uses      net/netip, strings.
// @reason    The host shape is a domain invariant because the SSRF guard
//
//	resolves what it is given — a host that is really a URL would move
//	the guard's target, and the rule "no scheme, no userinfo, no path"
//	is what keeps the two in step. It is separate from the aggregate
//	because AGENTS.md §1.1 caps a file at 250 lines and the aggregate
//	already carries its own transitions.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-19
package domain

import (
	"net/netip"
	"strings"
)

// ProxyProtocol is the closed set of protocols §7.11 admits. The set is closed
// because each member maps to one dialer the adapter can build; a protocol
// outside it would reach the adapter as an unbuildable target.
type ProxyProtocol string

const (
	ProxyProtocolHTTP   ProxyProtocol = "http"
	ProxyProtocolHTTPS  ProxyProtocol = "https"
	ProxyProtocolSOCKS5 ProxyProtocol = "socks5"
)

// ParseProxyProtocol validates a wire value against the closed set.
func ParseProxyProtocol(raw string) (ProxyProtocol, error) {
	protocol := ProxyProtocol(strings.ToLower(strings.TrimSpace(raw)))
	if err := validateProxyProtocol(protocol); err != nil {
		return "", err
	}
	return protocol, nil
}

// validateProxyProtocol is the closed-set check the constructor and every
// transition share, so a value cast past the parser still cannot be stored.
func validateProxyProtocol(protocol ProxyProtocol) error {
	switch protocol {
	case ProxyProtocolHTTP, ProxyProtocolHTTPS, ProxyProtocolSOCKS5:
		return nil
	default:
		return NewValidationError("protocol must be one of http, https, socks5")
	}
}

// validateProxyHost enforces that the host is a bare hostname or IP literal.
//
// A value carrying a scheme, userinfo, path, or port would make the guard's
// resolution target something other than what the operator sees, so the shape
// is refused rather than parsed apart. An all-digits-and-dots value that is not
// a valid IP literal is refused too: that is the decimal form of an IPv4
// address, and accepting it would hand the dialer an address the operator's
// panel never displayed (OWASP A01, IP obfuscation).
func validateProxyHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return NewValidationError("host is required")
	}
	if len(host) > 253 {
		return NewValidationError("host must be at most 253 characters")
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return nil
	}
	if strings.ContainsAny(host, "/@?#: \t\r\n\\") {
		return NewValidationError("host must be a bare hostname or IP address, without a scheme, credentials, path, or port")
	}
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return NewValidationError("host must contain only letters, digits, dots, dashes, and underscores")
		}
	}
	if strings.Trim(host, "0123456789.") == "" {
		return NewValidationError("host must not be a numeric address; write it in dotted form")
	}
	return nil
}

// validateProxyPort bounds the port to the TCP range.
func validateProxyPort(port int) error {
	if port < 1 || port > 65535 {
		return NewValidationError("port must be between 1 and 65535")
	}
	return nil
}
