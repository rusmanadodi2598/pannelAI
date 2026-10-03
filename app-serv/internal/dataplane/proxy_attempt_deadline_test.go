// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_attempt_deadline_test.go
// @for       The proxied attempt's context: the per-attempt deadline must bound
//
//	the outbound call exactly as it does on the direct path.
//
// @uses      context, net/http, net/http/httptest, net/url, provider, registry,
//
//	testing, time.
//
// @reason    A planned proxy walk clones the request with the context the dialer
//
//	was handed. Before this test that was the caller's context, so a
//	registry timeout_ms never bounded a proxied call: the stall ran to the
//	shared transport's 120 s header timeout instead of the attempt's own
//	deadline. The code the client sees is the discriminator.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-03
package dataplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// oneCandidatePlanner hands the dialer a single pool row, which is all the walk
// needs to take the proxied path.
type oneCandidatePlanner struct{ attempt domain.ProxyRouteAttempt }

func (p oneCandidatePlanner) Plan(context.Context, string, string) ([]domain.ProxyRouteAttempt, error) {
	return []domain.ProxyRouteAttempt{p.attempt}, nil
}

func (oneCandidatePlanner) ReportFailure(context.Context, string) {}

// TestProxiedAttemptHonoursTheProviderTimeout pins that a proxied call is
// bounded by the provider's timeout_ms rather than by the shared transport's
// header timeout. The proxy holds its headers until the client gives up, so the
// only thing that can end the call is the attempt deadline.
func TestProxiedAttemptHonoursTheProviderTimeout(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hold the headers until the client gives up, with a bound so the
		// server's Close cannot wait on a connection the client abandoned
		// without the close being noticed.
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	t.Cleanup(proxy.Close)

	shared := &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 2 * time.Second}}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, &envelopeConnector{
		Base: provider.Base{ID: "proxy-deadline-test", Auth: "apikey", Format: "openai"},
		url:  "http://upstream.invalid/v1/chat/completions",
	})
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	candidate, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatalf("parsing the proxy URL: %v", err)
	}
	transport, err := NewTransport(TransportDeps{
		Connectors: connectors,
		Client:     shared,
		Routes:     oneCandidatePlanner{attempt: domain.ProxyRouteAttempt{ID: "pool-1", URL: candidate}},
	})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}

	started := time.Now()
	_, err = transport.Do(context.Background(), Call{
		Provider: registry.Provider{
			ID:        "proxy-deadline-test",
			Transport: registry.Transport{TimeoutMS: 50},
		},
		Model:      registry.Model{ID: "m"},
		Body:       []byte(`{"model":"m"}`),
		Credential: provider.StaticKey("ep_1", "key_1", "k"),
	})
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("Do() = nil error, want the attempt deadline to end the call")
	}
	if got := AsError(err).Code; got != CodeUpstreamTimeout {
		t.Fatalf("code = %s, want %s (the attempt deadline must bound a proxied call)", got, CodeUpstreamTimeout)
	}
	if elapsed > time.Second {
		t.Fatalf("call ended after %v, want the 50 ms attempt deadline, not the 2 s header timeout", elapsed)
	}
}
