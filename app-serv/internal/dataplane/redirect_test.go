// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/redirect_test.go
// @for       The outbound client's redirect policy: an upstream 3xx is a failure, never a hop that carries the credential and the body elsewhere.
// @uses      context, net/http, net/http/httptest, provider, registry, sync/atomic, testing.
// @reason    Go strips only Authorization, Cookie and Www-Authenticate on a cross-host redirect; the registry declares credentials in custom headers (x-api-key, x-goog-api-key), and a 307/308 re-sends the body, so following a redirect hands an account's key to the redirect target.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package dataplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestUpstreamRedirectIsNotFollowed pins that a 3xx from an upstream is
// reported as a failure of that upstream: the redirect target must receive no
// request, and therefore no credential header and no body.
func TestUpstreamRedirectIsNotFollowed(t *testing.T) {
	var hits atomic.Int32
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("x-api-key") != "" {
			leaked.Store(true)
		}
	}))
	t.Cleanup(target.Close)

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL+"/moved")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirector.Close)

	connectors, err := provider.NewConnectors(provider.DefaultFactory, &envelopeConnector{
		Base: provider.Base{ID: "redirect-test", Auth: "apikey", Format: "openai"},
		url:  redirector.URL + "/v1/chat/completions",
	})
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: NewHTTPClient(HTTPClientDeps{})})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}

	_, err = transport.Do(context.Background(), Call{
		Provider: registry.Provider{
			ID:        "redirect-test",
			Transport: registry.Transport{Headers: map[string]string{"x-api-key": "upstream-secret"}},
		},
		Model: registry.Model{ID: "m"},
		Body:  []byte(`{"model":"m"}`),
	})
	if err == nil {
		t.Fatal("Do() = nil error, want the 307 reported as an upstream failure")
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("the redirect target received %d requests, want 0", got)
	}
	if leaked.Load() {
		t.Fatal("the upstream credential reached the redirect target")
	}
}
