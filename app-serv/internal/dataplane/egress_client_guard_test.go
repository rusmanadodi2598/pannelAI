// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/egress_client_guard_test.go
// @for       The rule that an outbound client is supplied, never invented: a constructor handed a nil client refuses it instead of building an unguarded one.
// @uses      internal/provider, net/http, net/url, testing.
// @reason    docs/RULLES/SSRF.md §2.1 and §3 say a nil egress client must fail rather than fall back. A client built inside these packages carries a plain net.Dialer and no proxy route, so a silent default would let the composition root look guarded while every call left unguarded, the same refusal draft 042 R07 already pins for the qoder connector.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// TestNewTransport_RefusesNilClient pins that a chat transport without the
// guarded client fails at wiring time rather than dialing with a plain one.
func TestNewTransport_RefusesNilClient(t *testing.T) {
	if _, err := NewTransport(TransportDeps{Connectors: &provider.Connectors{}}); err == nil {
		t.Fatal("NewTransport() accepted a nil client, want a refusal")
	}
}

// TestNewMediaTransport_RefusesNilClient pins the same refusal on the media
// half, which reaches embeddings, TTS and image hosts.
func TestNewMediaTransport_RefusesNilClient(t *testing.T) {
	if _, err := NewMediaTransport(nil, nil); err == nil {
		t.Fatal("NewMediaTransport() accepted a nil client, want a refusal")
	}
}

// TestProxyDialer_RefusesClientWithoutTransport pins that the per-candidate
// proxy client is cloned from a real transport. Falling back to
// http.DefaultTransport would route the attempt through a dialer nobody
// reviewed, one that also honours HTTPS_PROXY from the environment.
func TestProxyDialer_RefusesClientWithoutTransport(t *testing.T) {
	candidate, err := url.Parse("http://proxy.invalid:8080")
	if err != nil {
		t.Fatalf("parsing the candidate: %v", err)
	}
	dialer := &ProxyDialer{client: &http.Client{}}

	if _, err := dialer.clientFor(candidate); err == nil {
		t.Fatal("clientFor() built a route from a client with no transport, want a refusal")
	}
}
