// Command app-serv adapts the stateless credential check to HTTP.
//
// @file      cmd/app-serv/provider_validate_guard_test.go
// @for       The two cases about what a check does *not* send: no request to a
//
//	refused destination, and a chat probe that names a model.
//
// @uses      internal/domain, internal/netguard, internal/provider,
//
//	internal/service, context, net/http, net/http/httptest, testing.
//
// @reason    Review Focus 1 applies to this path as much as to the model read:
//
//	the destination is operator-supplied, so a refused address must be
//	reported as a refusal and never dialed. Keeping it apart from the
//	status tables keeps each file's reason readable.
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
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TestValidateNode_RefusesADeniedDestination is Review Focus 1 on the new path:
// a destination the guard refuses is reported as a refusal and is never dialed.
func TestValidateNode_RefusesADeniedDestination(t *testing.T) {
	var reached bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	guard, err := netguard.NewGuard(nil) // no allowlist: loopback is refused
	if err != nil {
		t.Fatalf("netguard.NewGuard() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("provider.NewConnectors() error = %v", err)
	}
	prober := newHTTPEndpointProber(staticProviderLookup{}, connectors, guard)

	outcome, err := prober.ValidateNode(context.Background(), service.CredentialCheck{
		BaseURL: server.URL, NodeType: string(domain.NodeOpenAICompatible), Credential: "sk-live-abcdef",
	})
	if err != nil {
		t.Fatalf("ValidateNode() error = %v", err)
	}
	if outcome.State != domain.EndpointTestFail {
		t.Fatalf("State = %q, want %q for a refused destination", outcome.State, domain.EndpointTestFail)
	}
	if outcome.Message == "" {
		t.Fatal("a refusal must carry the reason")
	}
	if reached {
		t.Fatal("the request reached a destination the guard refused")
	}
}

// TestValidateNode_ChatProbeCarriesAModelAndOneToken pins what the fallback
// sends: an upstream rejects an empty model before it reads the credential.
func TestValidateNode_ChatProbeCarriesAModelAndOneToken(t *testing.T) {
	cases := []struct {
		name      string
		modelID   string
		wantModel string
	}{
		{name: "a supplied model is named", modelID: "gpt-4o-mini", wantModel: "gpt-4o-mini"},
		{name: "no model falls back to a placeholder", modelID: "", wantModel: probeModelFallback},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := newValidateUpstream(t)
			up.modelsStatus.Store(http.StatusNotFound)
			prober := validateProber(t)

			if _, err := prober.ValidateNode(context.Background(), service.CredentialCheck{
				BaseURL: up.server.URL, NodeType: string(domain.NodeOpenAICompatible),
				Credential: "sk-live-abcdef", ModelID: tc.modelID,
			}); err != nil {
				t.Fatalf("ValidateNode() error = %v", err)
			}
			body, _ := up.lastChatBody.Load().(string)
			if !contains(body, `"model":"`+tc.wantModel+`"`) {
				t.Fatalf("the chat probe body = %q, want it to name %q", body, tc.wantModel)
			}
			if !contains(body, `"max_tokens":1`) {
				t.Fatalf("the chat probe body = %q, want max_tokens 1", body)
			}
		})
	}
}
