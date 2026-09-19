// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport.go
// @for       The transport's shape and the shared HTTP client it dials with.
// @uses      internal/provider, net, net/http, sort, time.
// @reason    SPEC-API-001 §4 fixes the timeouts (connect 10s, total 120s, no
//
//	total cap while streaming with a 300s idle read) and AGENTS.md §1.6
//	requires every outbound call to run under a context deadline. The
//	pool limits live here because one client serves every upstream call
//	(AGENTS.md §1.7), and its dialer is the seam the composition root
//	injects the egress guard through (OWASP A01). Nothing here branches
//	on a provider id: the URL and the credential placement come from
//	provider.Plugin, which is what makes a new provider a registry entry
//	rather than a change to shared code.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"net"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// Deadlines fixed by SPEC-API-001 §4. A registry entry may override the total
// and the idle read through timeout_ms and stall_timeout_ms; the connect deadline
// has no override because a TCP handshake has no provider-specific shape.
const (
	// ConnectTimeout bounds the TCP and TLS handshake.
	ConnectTimeout = 10 * time.Second
	// TotalTimeout bounds one non-streamed call end to end.
	TotalTimeout = 120 * time.Second
	// IdleTimeout bounds the gap between two reads of a streamed body. A stream
	// has no total cap because a long answer is not a failure, so the gap is
	// what detects a dead upstream.
	IdleTimeout = 300 * time.Second
	// maxUpstreamBodyBytes bounds a non-streamed response body, which is
	// buffered so the context can be released before the caller reads it.
	maxUpstreamBodyBytes = 32 << 20
	// errorBodyBytes bounds how much of an upstream error body is read, so a
	// provider answering with a huge page cannot be buffered.
	errorBodyBytes = 64 << 10
)

// SecretOpener decrypts a stored credential for exactly one request. It is an
// interface so the data plane never constructs a sealer with the process key:
// sealing and opening must share one implementation, or a value written by one
// path becomes unreadable by the other.
type SecretOpener interface {
	Open(sealed string) (string, error)
}

// Transport performs outbound calls through the plugin seam.
type Transport struct {
	connectors *provider.Connectors
	client     *http.Client
}

// TransportDeps holds the collaborators the transport needs.
type TransportDeps struct {
	// Connectors resolves a provider id to the plugin that handles it.
	Connectors *provider.Connectors
	// Client overrides the default HTTP client. The composition root passes the
	// guarded client from egress_wiring.go, so a chat call reaches no address the
	// egress guard refused; the default carries the §1.6 deadlines and explicit
	// pool limits.
	Client *http.Client
}

// NewTransport validates deps and returns a transport.
func NewTransport(deps TransportDeps) (*Transport, error) {
	if deps.Connectors == nil {
		return nil, internalError("provider connectors are required", nil)
	}
	client := deps.Client
	if client == nil {
		client = NewHTTPClient(HTTPClientDeps{})
	}
	return &Transport{connectors: deps.Connectors, client: client}, nil
}

// HTTPClientDeps holds the seams the shared upstream client is built from.
type HTTPClientDeps struct {
	// Dialer overrides the TCP dialer. The composition root passes the egress
	// guard's dialer, so the address actually reached is validated at connect
	// time (OWASP A01). A zero value keeps the plain dialer, which is what the
	// hermetic tests use: an httptest server is loopback, and the guard refuses
	// loopback unless the operator allowlists it.
	Dialer *net.Dialer
	// Proxy decides how one request reaches its destination: nil to dial it
	// directly, or the proxy to dial instead (SPEC-API-001 §7.11). The
	// composition root builds it from settings.network, so the routing decision
	// stays out of this package; a zero value dials direct.
	//
	// A proxied request never dials the destination, so a caller that routes
	// through a proxy owns validating the destination itself — the dialer's
	// guard only sees the proxy's address.
	Proxy func(*http.Request) (*url.URL, error)
}

// NewHTTPClient builds the HTTP client the gateway calls upstreams with.
//
// The pool limits are set explicitly rather than left at the library default
// (AGENTS.md §1.7): an unbounded pool fans out to a failing upstream as fast as
// to a healthy one, and the idle cap stops a long tail of providers from holding
// sockets open.
func NewHTTPClient(deps HTTPClientDeps) *http.Client {
	dialer := deps.Dialer
	if dialer == nil {
		dialer = &net.Dialer{Timeout: ConnectTimeout, KeepAlive: 30 * time.Second}
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			Proxy:                 deps.Proxy,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   16,
			MaxConnsPerHost:       64,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   ConnectTimeout,
			ResponseHeaderTimeout: TotalTimeout,
			ExpectContinueTimeout: time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}

// applyHeaders copies the registry entry's declared headers and the shape headers
// onto the outbound request. A connector may add to these; nothing here is
// provider-specific, which is why the entry carries them as data.
func applyHeaders(dst http.Header, call Call) {
	declared := call.Provider.Transport.Headers
	keys := make([]string, 0, len(declared))
	for key := range declared {
		keys = append(keys, key)
	}
	// Sorted so a test comparing the outbound request is not order-dependent.
	sort.Strings(keys)
	for _, key := range keys {
		dst.Set(key, declared[key])
	}
	if dst.Get("Content-Type") == "" {
		dst.Set("Content-Type", "application/json")
	}
	if call.Stream {
		dst.Set("Accept", "text/event-stream")
	}
}
