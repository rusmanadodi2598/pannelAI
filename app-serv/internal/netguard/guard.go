// Package netguard validates an outbound destination before the gateway
// connects to it (OWASP A01 SSRF, docs/RULLES/OWASP.md).
//
// @file      internal/netguard/guard.go
// @for       One generalized egress guard: resolve, validate every resolved
//
//	address, and re-check at connect time.
//
// @uses      context, errors, fmt, net, net/netip, strings, syscall, time.
// @reason    A proxy candidate makes the server dial an address an operator
//
//	typed. Checking the string would be the pattern the OWASP rules
//	blacklist: the check must run on the resolved IP, and again on the
//	address the dialer is about to reach, or a DNS answer that changes
//	between the two calls walks straight through. One guard, used by
//	every such dialer, is what makes the rule hold for the class.
//
//	Two tiers, because a self-hosted gateway may legitimately reach its
//	own network: addresses that are never a host (link-local, multicast,
//	the reserved ranges, CGNAT) are refused outright, while loopback and
//	private ranges are refused unless the operator names them in
//	EGRESS_ALLOWED_TARGETS — default deny, allowlist to permit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"syscall"
	"time"
)

// ErrDenied marks a destination the guard refused. A caller can test for it to
// tell a policy refusal from a resolution or transport failure.
var ErrDenied = errors.New("outbound destination denied")

// DeniedError carries why an address was refused. The reason is what a panel
// shows an operator, so it is a field rather than part of a wrapped chain.
type DeniedError struct {
	Reason string
}

func (e *DeniedError) Error() string { return ErrDenied.Error() + ": " + e.Reason }

// Unwrap keeps errors.Is(err, ErrDenied) working for a typed refusal.
func (e *DeniedError) Unwrap() error { return ErrDenied }

// Reason returns the refusal's reason when err is a refusal, and "" otherwise.
func Reason(err error) string {
	var denied *DeniedError
	if errors.As(err, &denied) {
		return denied.Reason
	}
	return ""
}

// Resolver is the DNS seam, so a test can drive every answer a name returns
// without touching the network.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Guard validates destinations against the two tiers described above.
type Guard struct {
	resolver Resolver
	allowed  []netip.Prefix
}

// NewGuard parses the operator allowlist and returns a guard over the system
// resolver. Each entry is a CIDR prefix or a single IP address; a malformed
// entry is an error, so a typo fails the boot rather than silently widening
// what the gateway may reach.
func NewGuard(allowed []string) (*Guard, error) {
	return NewGuardWithResolver(net.DefaultResolver, allowed)
}

// NewGuardWithResolver is NewGuard with the resolver injected, which is what
// the tests drive.
func NewGuardWithResolver(resolver Resolver, allowed []string) (*Guard, error) {
	if resolver == nil {
		return nil, errors.New("netguard: a resolver is required")
	}
	prefixes := make([]netip.Prefix, 0, len(allowed))
	for _, entry := range allowed {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		address, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("netguard: %q is neither a CIDR prefix nor an IP address", entry)
		}
		prefixes = append(prefixes, netip.PrefixFrom(address, address.BitLen()))
	}
	return &Guard{resolver: resolver, allowed: prefixes}, nil
}

// CheckHost resolves the host once and validates every address it resolves to.
//
// Every answer is checked, and one denied answer denies the host: a name that
// answers with both a public and a private address is exactly the shape a
// rebinding attempt has, so the guard fails closed rather than picking the
// address that passes.
func (g *Guard) CheckHost(ctx context.Context, host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("%w: the host is empty", ErrDenied)
	}
	addresses, err := g.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", host, err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("%w: %s resolved to no address", ErrDenied, host)
	}
	for _, address := range addresses {
		if err := g.CheckIP(address); err != nil {
			return fmt.Errorf("%s: %w", host, err)
		}
	}
	return nil
}

// CheckIP validates one resolved address.
func (g *Guard) CheckIP(ip netip.Addr) error {
	if !ip.IsValid() {
		return fmt.Errorf("%w: an invalid address", ErrDenied)
	}
	// ::ffff:127.0.0.1 is loopback wearing an IPv6 spelling, so it is unwrapped
	// before any range check.
	ip = ip.Unmap()
	// Transition addresses carry an IPv4 address inside them (NAT64, 6to4,
	// Teredo). The embedded address is what the connection reaches, so it is
	// what the range rules apply to.
	if embedded, ok := embeddedIPv4(ip); ok {
		if err := g.CheckIP(embedded); err != nil {
			return err
		}
	}
	// The never-a-host tier is checked first, so a wide allowlist (an operator
	// who wrote 0.0.0.0/0) cannot re-open the ranges that are not a host at
	// all — the metadata address among them.
	if reason := neverAHost(ip); reason != "" {
		return &DeniedError{Reason: reason}
	}
	if g.permits(ip) {
		return nil
	}
	if reason := needsAllowlist(ip); reason != "" {
		return &DeniedError{Reason: reason + "; add it to EGRESS_ALLOWED_TARGETS if it is your own proxy"}
	}
	return nil
}

// Control is the net.Dialer hook. It runs after the name has been resolved and
// before the connection is made, so it validates the address actually being
// reached — the second half of the anti-rebinding rule.
func (g *Guard) Control(network, address string, _ syscall.RawConn) error {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: the dial address %q is malformed", ErrDenied, address)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: the dial address %q is not an IP literal", ErrDenied, address)
	}
	return g.CheckIP(ip)
}

// NewDialer returns a dialer whose Control hook is this guard, so a caller
// cannot wire the timeout and forget the check.
func (g *Guard) NewDialer(timeout, keepAlive time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout, KeepAlive: keepAlive, Control: g.Control}
}

// permits reports whether the operator explicitly allowlisted the address.
func (g *Guard) permits(ip netip.Addr) bool {
	for _, prefix := range g.allowed {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
