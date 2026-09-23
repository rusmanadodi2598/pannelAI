// Command app-serv adapts the stateless credential check to HTTP.
//
// @file      cmd/app-serv/provider_validate_test.go
// @for       The two reference rules the stateless check implements: the models →
//
//	chat fallback, and the Anthropic status rule.
//
// @uses      internal/domain, internal/netguard, internal/provider,
//
//	internal/registry, internal/service, context, net/http,
//	net/http/httptest, sync/atomic, testing.
//
// @reason    Draft 017 §4.6 names both rules as the ones a naive implementation
//
//	gets wrong, and both are invisible to a status-only test: a fallback
//	that never fires still reports the right answer for a server that has
//	`/models`, and an Anthropic rule narrowed to 2xx still reports the right
//	answer for a working key. Each case therefore asserts the *request path*
//	as well as the outcome.
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

// validateUpstream is a fake compatible server that answers a fixed status per
// path and counts the requests it saw, so a test can assert which path was tried.
type validateUpstream struct {
	server *httptest.Server
	// modelsStatus and chatStatus are the statuses each path answers.
	modelsStatus atomic.Int64
	chatStatus   atomic.Int64
	modelsHits   atomic.Int64
	chatHits     atomic.Int64
	// lastChatBody records what the fallback sent.
	lastChatBody atomic.Value
}

func newValidateUpstream(t *testing.T) *validateUpstream {
	t.Helper()
	up := &validateUpstream{}
	up.modelsStatus.Store(http.StatusOK)
	up.chatStatus.Store(http.StatusOK)
	up.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			up.modelsHits.Add(1)
			w.WriteHeader(int(up.modelsStatus.Load()))
			return
		}
		up.chatHits.Add(1)
		if r.Body != nil {
			body := make([]byte, 256)
			n, _ := r.Body.Read(body)
			up.lastChatBody.Store(string(body[:n]))
		}
		w.WriteHeader(int(up.chatStatus.Load()))
	}))
	t.Cleanup(up.server.Close)
	return up
}

// validateProber builds the adapter pointed at a test server, with loopback
// allowlisted so the destination is reachable.
func validateProber(t *testing.T) *httpEndpointProber {
	t.Helper()
	guard, err := netguard.NewGuard([]string{"127.0.0.1/32", "::1/128"})
	if err != nil {
		t.Fatalf("netguard.NewGuard() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("provider.NewConnectors() error = %v", err)
	}
	return newHTTPEndpointProber(staticProviderLookup{}, connectors, guard)
}

// staticProviderLookup resolves no provider, so only the node path is exercised.
type staticProviderLookup struct{}

func (staticProviderLookup) Provider(string) (registry.Provider, bool) {
	return registry.Provider{}, false
}

// TestValidateNode_ModelsThenChatFallback is the fallback rule as a table.
func TestValidateNode_ModelsThenChatFallback(t *testing.T) {
	cases := []struct {
		name         string
		modelsStatus int
		chatStatus   int
		wantState    string
		wantMethod   string
		wantModels   int64
		wantChat     int64
	}{
		{
			name:         "a 200 on /models proves the key without a chat probe",
			modelsStatus: http.StatusOK, chatStatus: http.StatusOK,
			wantState: domain.EndpointTestOK, wantMethod: service.ProbeMethodModels, wantModels: 1, wantChat: 0,
		},
		{
			name:         "a 404 on /models falls back to chat, which proves the key",
			modelsStatus: http.StatusNotFound, chatStatus: http.StatusOK,
			wantState: domain.EndpointTestOK, wantMethod: service.ProbeMethodChat, wantModels: 1, wantChat: 1,
		},
		{
			name:         "a 404 on both is a failure that names the chat path",
			modelsStatus: http.StatusNotFound, chatStatus: http.StatusNotFound,
			wantState: domain.EndpointTestFail, wantMethod: service.ProbeMethodChat, wantModels: 1, wantChat: 1,
		},
		{
			name:         "a 401 on /models is rejected and is not retried",
			modelsStatus: http.StatusUnauthorized, chatStatus: http.StatusOK,
			wantState: domain.EndpointTestFail, wantMethod: service.ProbeMethodModels, wantModels: 1, wantChat: 0,
		},
		{
			name:         "a 403 on /models is rejected and is not retried",
			modelsStatus: http.StatusForbidden, chatStatus: http.StatusOK,
			wantState: domain.EndpointTestFail, wantMethod: service.ProbeMethodModels, wantModels: 1, wantChat: 0,
		},
		{
			name:         "a 500 on /models falls back to chat",
			modelsStatus: http.StatusInternalServerError, chatStatus: http.StatusOK,
			wantState: domain.EndpointTestOK, wantMethod: service.ProbeMethodChat, wantModels: 1, wantChat: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := newValidateUpstream(t)
			up.modelsStatus.Store(int64(tc.modelsStatus))
			up.chatStatus.Store(int64(tc.chatStatus))
			prober := validateProber(t)

			outcome, err := prober.ValidateNode(context.Background(), service.CredentialCheck{
				BaseURL: up.server.URL, NodeType: string(domain.NodeOpenAICompatible), Credential: "sk-live-abcdef",
			})
			if err != nil {
				t.Fatalf("ValidateNode() error = %v", err)
			}
			if outcome.State != tc.wantState {
				t.Fatalf("State = %q, want %q (message: %s)", outcome.State, tc.wantState, outcome.Message)
			}
			if outcome.Method != tc.wantMethod {
				t.Fatalf("Method = %q, want %q", outcome.Method, tc.wantMethod)
			}
			if got := up.modelsHits.Load(); got != tc.wantModels {
				t.Fatalf("the models path was asked %d times, want %d", got, tc.wantModels)
			}
			if got := up.chatHits.Load(); got != tc.wantChat {
				t.Fatalf("the chat path was asked %d times, want %d", got, tc.wantChat)
			}
		})
	}
}

// TestValidateNode_AnthropicAcceptsAnythingBut401And403 is the second rule.
func TestValidateNode_AnthropicAcceptsAnythingBut401And403(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		wantState string
	}{
		{name: "a 200 is ok", status: http.StatusOK, wantState: domain.EndpointTestOK},
		{name: "a 400 means the request was wrong, not the key", status: http.StatusBadRequest, wantState: domain.EndpointTestOK},
		{name: "a 404 means the path was wrong, not the key", status: http.StatusNotFound, wantState: domain.EndpointTestOK},
		{name: "a 529 means the provider is overloaded, not the key", status: 529, wantState: domain.EndpointTestOK},
		{name: "a 401 is a rejected key", status: http.StatusUnauthorized, wantState: domain.EndpointTestFail},
		{name: "a 403 is a rejected key", status: http.StatusForbidden, wantState: domain.EndpointTestFail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := newValidateUpstream(t)
			up.modelsStatus.Store(int64(tc.status))
			prober := validateProber(t)

			outcome, err := prober.ValidateNode(context.Background(), service.CredentialCheck{
				BaseURL: up.server.URL, NodeType: string(domain.NodeAnthropicCompatible), Credential: "sk-live-abcdef",
			})
			if err != nil {
				t.Fatalf("ValidateNode() error = %v", err)
			}
			if outcome.State != tc.wantState {
				t.Fatalf("State = %q, want %q (status %d, message: %s)", outcome.State, tc.wantState, tc.status, outcome.Message)
			}
			// An Anthropic check never falls back: the rule already decides, and
			// a second request would only spend the operator's quota.
			if got := up.chatHits.Load(); got != 0 {
				t.Fatalf("the Anthropic path fell back to chat %d times, want 0", got)
			}
		})
	}
}
