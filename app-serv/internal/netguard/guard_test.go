// Package netguard validates an outbound destination before the gateway
// connects to it (OWASP A01 SSRF, docs/RULLES/OWASP.md).
//
// @file      internal/netguard/guard_test.go
// @for       The egress guard's range rules, resolution rules, and dial-time
//
//	re-check, as tables over the obfuscation classes.
//
// @uses      testing, context, errors, net/netip, strings, syscall.
// @reason    The OWASP rules require a parameterized table rather than a single
//
//	example payload, and require a benign case that must still be allowed:
//	a guard that blocks everything passes a one-case test. The tables
//	below vary the spelling of an address — decimal-free literals,
//	IPv6-mapped forms, and transition addresses that embed IPv4 — because
//	those are the shapes a bypass attempt takes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package netguard

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// stubResolver answers with a configured address set, so a hostname case never
// touches DNS.
type stubResolver struct {
	answers map[string][]netip.Addr
	err     error
}

func (s stubResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.answers[host], nil
}

// mustGuard builds a guard over the stub resolver or fails the test.
func mustGuard(t *testing.T, allowed []string) *Guard {
	t.Helper()
	guard, err := NewGuardWithResolver(stubResolver{answers: map[string][]netip.Addr{}}, allowed)
	if err != nil {
		t.Fatalf("NewGuardWithResolver() error = %v", err)
	}
	return guard
}

// TestGuard_CheckIP pins the address classes: what is always refused, what the
// allowlist can permit, and the benign public addresses that must pass.
func TestGuard_CheckIP(t *testing.T) {
	cases := []struct {
		name    string
		address string
		allowed []string
		wantOK  bool
		want    string
	}{
		{name: "a public address", address: "93.184.216.34", wantOK: true},
		{name: "another public address", address: "8.8.8.8", wantOK: true},
		{name: "a public IPv6 address", address: "2606:4700:4700::1111", wantOK: true},
		{name: "loopback", address: "127.0.0.1", want: "loopback"},
		{name: "loopback, allowlisted", address: "127.0.0.1", allowed: []string{"127.0.0.0/8"}, wantOK: true},
		{name: "the IPv6 loopback", address: "::1", want: "loopback"},
		{name: "IPv4-mapped loopback", address: "::ffff:127.0.0.1", want: "loopback"},
		{name: "a private address", address: "10.0.0.5", want: "private"},
		{name: "a 172.16 address", address: "172.16.0.9", want: "private"},
		{name: "a 192.168 address", address: "192.168.1.1", want: "private"},
		{name: "a private address, allowlisted", address: "192.168.1.1", allowed: []string{"192.168.0.0/16"}, wantOK: true},
		{name: "a unique-local address", address: "fd00::1", want: "private"},
		{name: "a unique-local address, allowlisted", address: "fd00::1", allowed: []string{"fd00::/8"}, wantOK: true},
		{name: "the metadata address", address: "169.254.169.254", want: "link-local"},
		{name: "the metadata address, allowlisted", address: "169.254.169.254", allowed: []string{"0.0.0.0/0"}, want: "link-local"},
		{name: "a link-local address", address: "fe80::1", want: "link-local"},
		{name: "the Alibaba metadata address", address: "100.100.100.200", allowed: []string{"0.0.0.0/0"}, want: "carrier-grade NAT"},
		{name: "the unspecified address", address: "0.0.0.0", want: "unspecified"},
		{name: "a multicast address", address: "224.0.0.1", want: "multicast"},
		{name: "the broadcast address", address: "255.255.255.255", want: "broadcast"},
		{name: "the reserved range", address: "240.0.0.1", want: "reserved"},
		{name: "the this-network range", address: "0.1.2.3", want: "this-network"},
		{name: "a NAT64 address embedding loopback", address: "64:ff9b::7f00:1", want: "loopback"},
		{name: "a NAT64 address embedding a public address", address: "64:ff9b::808:808", wantOK: true},
		{name: "a 6to4 address embedding loopback", address: "2002:7f00:1::", want: "loopback"},
		{name: "a Teredo address embedding loopback", address: "2001:0:4136:e378:8000:63bf:80ff:fffe", want: "loopback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			guard := mustGuard(t, tc.allowed)
			err := guard.CheckIP(netip.MustParseAddr(tc.address))
			if tc.wantOK {
				if err != nil {
					t.Fatalf("CheckIP(%s) error = %v, want allowed", tc.address, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckIP(%s) allowed an address that must be refused", tc.address)
			}
			if !errors.Is(err, ErrDenied) {
				t.Fatalf("CheckIP(%s) error = %v, want ErrDenied", tc.address, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("CheckIP(%s) error = %v, want it to mention %q", tc.address, err, tc.want)
			}
		})
	}
}

// TestGuard_CheckHost pins the resolution rules: every answer is checked, so a
// name that answers with a public and a private address fails closed.
func TestGuard_CheckHost(t *testing.T) {
	addresses := func(values ...string) []netip.Addr {
		out := make([]netip.Addr, 0, len(values))
		for _, value := range values {
			out = append(out, netip.MustParseAddr(value))
		}
		return out
	}
	cases := []struct {
		name    string
		answers map[string][]netip.Addr
		host    string
		wantOK  bool
	}{
		{name: "a public name", host: "proxy.example.com", answers: map[string][]netip.Addr{
			"proxy.example.com": addresses("93.184.216.34", "2606:4700:4700::1111"),
		}, wantOK: true},
		{name: "a name resolving to a private address", host: "internal.example.com", answers: map[string][]netip.Addr{
			"internal.example.com": addresses("10.0.0.5"),
		}},
		{name: "a name answering public and private", host: "split.example.com", answers: map[string][]netip.Addr{
			"split.example.com": addresses("93.184.216.34", "192.168.1.1"),
		}},
		{name: "a name resolving to nothing", host: "empty.example.com", answers: map[string][]netip.Addr{
			"empty.example.com": {},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			guard, err := NewGuardWithResolver(stubResolver{answers: tc.answers}, nil)
			if err != nil {
				t.Fatalf("NewGuardWithResolver() error = %v", err)
			}
			err = guard.CheckHost(context.Background(), tc.host)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("CheckHost(%s) error = %v, want allowed", tc.host, err)
				}
				return
			}
			if !errors.Is(err, ErrDenied) {
				t.Fatalf("CheckHost(%s) error = %v, want ErrDenied", tc.host, err)
			}
		})
	}
}

// TestGuard_CheckHostReportsResolutionFailure pins that a resolver failure is
// not mistaken for a policy refusal, so a caller can tell the two apart.
func TestGuard_CheckHostReportsResolutionFailure(t *testing.T) {
	guard, err := NewGuardWithResolver(stubResolver{err: errors.New("the resolver is unreachable")}, nil)
	if err != nil {
		t.Fatalf("NewGuardWithResolver() error = %v", err)
	}
	err = guard.CheckHost(context.Background(), "proxy.example.com")
	if err == nil {
		t.Fatal("CheckHost() = nil error, want the resolver failure")
	}
	if errors.Is(err, ErrDenied) {
		t.Fatalf("CheckHost() error = %v, want a resolution failure rather than a policy refusal", err)
	}
}

// TestGuard_Control pins the dial-time re-check: it sees the resolved address,
// which is what closes the window between validating a name and connecting.
func TestGuard_Control(t *testing.T) {
	guard := mustGuard(t, nil)
	cases := []struct {
		name    string
		network string
		address string
		wantErr bool
	}{
		{name: "a public address", network: "tcp", address: "93.184.216.34:8080"},
		{name: "loopback", network: "tcp", address: "127.0.0.1:3128", wantErr: true},
		{name: "a private address", network: "tcp4", address: "10.0.0.5:3128", wantErr: true},
		{name: "an unparseable address", network: "tcp", address: "proxy.example.com:3128", wantErr: true},
		{name: "a malformed address", network: "tcp", address: "127.0.0.1", wantErr: true},
		{name: "a non-TCP network is left alone", network: "udp", address: "127.0.0.1:53"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := guard.Control(tc.network, tc.address, nil)
			if tc.wantErr && err == nil {
				t.Fatalf("Control(%s, %s) allowed a denied address", tc.network, tc.address)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Control(%s, %s) error = %v, want allowed", tc.network, tc.address, err)
			}
		})
	}
}

// TestNewGuard_AllowlistParsing pins that the allowlist accepts CIDRs and bare
// addresses and refuses a typo, so a mistyped entry cannot widen the policy.
func TestNewGuard_AllowlistParsing(t *testing.T) {
	cases := []struct {
		name    string
		allowed []string
		wantErr bool
	}{
		{name: "CIDR prefixes", allowed: []string{"10.0.0.0/8", "fd00::/8"}},
		{name: "a bare address", allowed: []string{"127.0.0.1"}},
		{name: "blank entries are ignored", allowed: []string{"", "  "}},
		{name: "a typo", allowed: []string{"10.0.0.0/33"}, wantErr: true},
		{name: "a hostname", allowed: []string{"proxy.example.com"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewGuardWithResolver(stubResolver{}, tc.allowed)
			if tc.wantErr && err == nil {
				t.Fatalf("NewGuardWithResolver(%v) accepted a malformed allowlist", tc.allowed)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("NewGuardWithResolver(%v) error = %v", tc.allowed, err)
			}
		})
	}
	if _, err := NewGuardWithResolver(nil, nil); err == nil {
		t.Fatal("NewGuardWithResolver(nil, nil) = nil error, want a missing resolver to be refused")
	}
}
