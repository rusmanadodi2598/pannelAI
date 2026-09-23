// Command app-serv adapts the stateless credential check to HTTP.
//
// @file      cmd/app-serv/provider_validate_plan_test.go
// @for       The provider path's per-format plan: a declared URL, a derived one,
//
//	and the Anthropic-wire POST that serves no model list.
//
// @uses      internal/domain, internal/netguard, internal/provider,
//
//	internal/registry, internal/service, context, net/http,
//	net/http/httptest, testing.
//
// @reason    Draft 017 §4.2's second consequence is that 76 providers answered
//
//	"this provider declares no validation endpoint". The plan closes it per
//	FORMAT, and the Anthropic wire is the case that cannot be closed by
//	appending a path: its base IS the messages endpoint. This test drives
//	the real adapter over a test server to prove the POST goes to that
//	exact URL and the Anthropic status rule decides the answer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// messagesUpstream is a fake Anthropic-wire upstream that answers one status and
// records the path and method it was asked with.
type messagesUpstream struct {
	server *httptest.Server
	status atomic.Int64
	path   atomic.Value
	method atomic.Value
	hits   atomic.Int64
}

func newMessagesUpstream(t *testing.T, status int) *messagesUpstream {
	t.Helper()
	up := &messagesUpstream{}
	up.status.Store(int64(status))
	up.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up.hits.Add(1)
		up.path.Store(r.URL.Path)
		up.method.Store(r.Method)
		w.WriteHeader(int(up.status.Load()))
	}))
	t.Cleanup(up.server.Close)
	return up
}

// planProber builds the adapter over a provider lookup that resolves one entry.
func planProber(t *testing.T, entry registry.Provider) *httpEndpointProber {
	t.Helper()
	guard, err := netguard.NewGuard([]string{"127.0.0.1/32", "::1/128"})
	if err != nil {
		t.Fatalf("netguard.NewGuard() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("provider.NewConnectors() error = %v", err)
	}
	return newHTTPEndpointProber(fixedProviderLookup{entry: entry}, connectors, guard)
}

// fixedProviderLookup resolves one entry, so the provider path is reachable
// without the embedded registry.
type fixedProviderLookup struct{ entry registry.Provider }

func (l fixedProviderLookup) Provider(name string) (registry.Provider, bool) {
	if name == l.entry.ID {
		return l.entry, true
	}
	return registry.Provider{}, false
}

// TestValidateProvider_AnthropicWirePostsToTheMessagesEndpoint is the case a
// path-appending rule cannot serve: the base IS the endpoint.
func TestValidateProvider_AnthropicWirePostsToTheMessagesEndpoint(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		wantState string
	}{
		{name: "a 200 is ok", status: http.StatusOK, wantState: domain.EndpointTestOK},
		{name: "a 400 proves the key was accepted", status: http.StatusBadRequest, wantState: domain.EndpointTestOK},
		{name: "a 529 proves the key was accepted", status: 529, wantState: domain.EndpointTestOK},
		{name: "a 401 is a rejected key", status: http.StatusUnauthorized, wantState: domain.EndpointTestFail},
		{name: "a 403 is a rejected key", status: http.StatusForbidden, wantState: domain.EndpointTestFail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := newMessagesUpstream(t, tc.status)
			entry := registry.Provider{
				ID: "claude", Category: "apikey", AuthType: registry.AuthAPIKey,
				Transport: registry.Transport{Format: "claude", BaseURL: up.server.URL + "/v1/messages"},
			}
			prober := planProber(t, entry)

			outcome, err := prober.ValidateProvider(context.Background(), service.CredentialCheck{
				ProviderID: "claude", Credential: "sk-live-abcdef",
			})
			if err != nil {
				t.Fatalf("ValidateProvider() error = %v", err)
			}
			if outcome.State != tc.wantState {
				t.Fatalf("State = %q, want %q (status %d, message: %s)", outcome.State, tc.wantState, tc.status, outcome.Message)
			}
			if got := up.method.Load(); got != http.MethodPost {
				t.Fatalf("method = %v, want POST: this wire serves no model list to read", got)
			}
			if got := up.path.Load(); got != "/v1/messages" {
				t.Fatalf("path = %v, want /v1/messages: the base IS the endpoint, so nothing is appended", got)
			}
			if got := up.hits.Load(); got != 1 {
				t.Fatalf("the upstream was asked %d times, want 1", got)
			}
		})
	}
}

// TestValidateProvider_DerivedModelsPath is the second rule: a chat-path base
// yields a models path, which is what took coverage from 18 to 37.
func TestValidateProvider_DerivedModelsPath(t *testing.T) {
	var askedPath atomic.Value
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		askedPath.Store(r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	entry := registry.Provider{
		ID: "openai", Category: "apikey", AuthType: registry.AuthAPIKey,
		Transport: registry.Transport{Format: "openai", BaseURL: up.URL + "/v1/chat/completions"},
	}
	prober := planProber(t, entry)

	outcome, err := prober.ValidateProvider(context.Background(), service.CredentialCheck{
		ProviderID: "openai", Credential: "sk-live-abcdef",
	})
	if err != nil {
		t.Fatalf("ValidateProvider() error = %v", err)
	}
	if outcome.State != domain.EndpointTestOK {
		t.Fatalf("State = %q, want ok", outcome.State)
	}
	if outcome.Method != service.ProbeMethodModels {
		t.Fatalf("Method = %q, want %q", outcome.Method, service.ProbeMethodModels)
	}
	if got := askedPath.Load(); got != "/v1/models" {
		t.Fatalf("path = %v, want /v1/models derived from the chat base", got)
	}
}

// TestValidateProvider_NoPlanIsAnAnswerNotAFault pins the uncovered case: a base
// naming neither a chat path nor a models path reports the reason rather than
// guessing a URL, because a guess produces a 404 that reads like a bad key.
func TestValidateProvider_NoPlanIsAnAnswerNotAFault(t *testing.T) {
	entry := registry.Provider{
		ID: "opaque", Category: "apikey", AuthType: registry.AuthAPIKey,
		Transport: registry.Transport{Format: "openai", BaseURL: "https://opaque.test/whatever"},
	}
	prober := planProber(t, entry)

	outcome, err := prober.ValidateProvider(context.Background(), service.CredentialCheck{
		ProviderID: "opaque", Credential: "sk-live-abcdef",
	})
	if err != nil {
		t.Fatalf("ValidateProvider() error = %v, want an answer", err)
	}
	if outcome.State != domain.EndpointTestFail {
		t.Fatalf("State = %q, want fail", outcome.State)
	}
	if outcome.Message == "" {
		t.Fatal("an unplannable provider must say why it cannot be checked")
	}
}

// TestValidateProvider_UnknownProviderIsAFault pins that a genuine configuration
// fault stays an error rather than being reported as a probe result.
func TestValidateProvider_UnknownProviderIsAFault(t *testing.T) {
	prober := planProber(t, registry.Provider{ID: "openai"})
	if _, err := prober.ValidateProvider(context.Background(), service.CredentialCheck{
		ProviderID: "no-such-provider",
	}); err == nil {
		t.Fatal("ValidateProvider() returned no error for a provider the registry does not know")
	}
}
