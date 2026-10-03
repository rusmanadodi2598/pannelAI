// Package clientip answers the one question the rate limiters ask: which
// client address is this request really from.
//
// @file      internal/clientip/clientip_test.go
// @for       The trusted-proxy rule: a forwarded chain is read only when the
//
//	direct peer is a proxy the operator named.
//
// @uses      net, net/http, testing.
// @reason    R20 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: the limiter bucketed
//
//	every client behind a reverse proxy into one address, and reading
//	X-Forwarded-For without a trust boundary would hand the bucket to
//	any caller who can set a header. Both halves of that rule are
//	pinned here: the chain is read only past a trusted peer, and a
//	forgeable header is refused from any other peer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-03
package clientip

import (
	"net"
	"testing"
)

// mustTrusted compiles the trust set a case needs, failing the test rather
// than installing a set nobody can reason about.
func mustTrusted(t *testing.T, cidrs []string) []*net.IPNet {
	t.Helper()
	trusted, err := ParseTrusted(cidrs)
	if err != nil {
		t.Fatalf("ParseTrusted() error = %v", err)
	}
	return trusted
}

// TestAddress_TrustedProxyReadsTheForwardedChain pins that a request from a
// trusted proxy is bucketed by the rightmost address in the chain that is not
// itself a trusted proxy, so a chained deployment resolves to the real client.
func TestAddress_TrustedProxyReadsTheForwardedChain(t *testing.T) {
	trusted := mustTrusted(t, []string{"10.0.0.0/8"})

	got := Address("10.0.0.9:41230", "203.0.113.7, 10.0.0.9", trusted)
	if got != "203.0.113.7" {
		t.Fatalf("address = %q, want the rightmost untrusted hop", got)
	}
}

// TestAddress_UntrustedPeerIgnoresTheHeader pins that a caller the operator has
// not named as a proxy cannot choose its own bucket by setting the header.
func TestAddress_UntrustedPeerIgnoresTheHeader(t *testing.T) {
	trusted := mustTrusted(t, []string{"10.0.0.0/8"})

	got := Address("203.0.113.5:44301", "9.9.9.9", trusted)
	if got != "203.0.113.5" {
		t.Fatalf("address = %q, want the peer the connection came from", got)
	}
}

// TestAddress_NoTrustedProxiesKeepsThePeer pins the default: with no
// TRUSTED_PROXY_CIDRS configured the header is never read, which is the
// behaviour every deployment had before this package existed.
func TestAddress_NoTrustedProxiesKeepsThePeer(t *testing.T) {
	got := Address("203.0.113.5:44301", "9.9.9.9", nil)
	if got != "203.0.113.5" {
		t.Fatalf("address = %q, want the peer with no trusted proxies configured", got)
	}
}

// TestAddress_AllTrustedHopsFallBackToThePeer pins that a chain of nothing but
// trusted proxies still answers with an address the operator can hold
// accountable — the proxy itself — rather than an empty bucket.
func TestAddress_AllTrustedHopsFallBackToThePeer(t *testing.T) {
	trusted := mustTrusted(t, []string{"10.0.0.0/8"})

	got := Address("10.0.0.9:41230", "10.0.0.2, 10.0.0.9", trusted)
	if got != "10.0.0.9" {
		t.Fatalf("address = %q, want the trusted peer when every hop is trusted", got)
	}
}

// TestParseTrusted_RefusesGarbage pins that a malformed CIDR fails
// configuration rather than installing a trust set nobody can reason about.
func TestParseTrusted_RefusesGarbage(t *testing.T) {
	if _, err := ParseTrusted([]string{"10.0.0.0/8", "not-a-cidr"}); err == nil {
		t.Fatal("ParseTrusted(garbage) = nil error, want a refusal")
	}
}
