// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_project_test.go
// @for       The project a Google login stores alongside its tokens, and what a failed lookup may not do.
// @uses      context, testing, time, internal/registry.
// @reason    The tokens and the Cloud project arrive from two different endpoints, and only one of them is the authorization the operator consented to. A login that cannot resolve a project must still land the account, because the credential is what recovers the project later; a login that silently drops it leaves every quota and inference call scoped to nothing. Both halves are pinned here, plus the rule that an entry declaring no bootstrap is never asked one.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package service

import (
	"context"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"testing"
)

// fakeProjectResolver answers the bootstrap with a scripted project and records
// the calls it received, so a test can prove the lookup was made for the right
// provider and token rather than merely that a value arrived.
type fakeProjectResolver struct {
	project string
	ok      bool
	calls   []string
}

func (f *fakeProjectResolver) ResolveProject(_ context.Context, providerID, accessToken string) (string, bool) {
	f.calls = append(f.calls, providerID+" "+accessToken)
	return f.project, f.ok
}

func providerWithBootstrap(id string) registry.Provider {
	provider := providerWithIdentity(id)
	provider.OAuth.LoadCodeAssistURL = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	provider.OAuth.LoadCodeAssistUA = "antigravity/ide/2.11.0 darwin/arm64"
	return provider
}

func TestOAuthCallbackStoresTheProjectTheBootstrapNamed(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithBootstrap("gc"))
	resolver := &fakeProjectResolver{project: "proj-9", ok: true}
	fixture.service.projects = resolver

	connected, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "gc", Code: "the-code", State: startFlow(t, fixture, "gc"),
	})
	if err != nil {
		t.Fatalf("Callback(): %v", err)
	}
	if got := connected.Endpoint.OAuth().ProjectID(); got != "proj-9" {
		t.Fatalf("stored project = %q, want the project the resolver named", got)
	}
	// The lookup runs on the token the vendor just issued, not on a stored one.
	if len(resolver.calls) != 1 || resolver.calls[0] != "gc at-issued" {
		t.Fatalf("resolver calls = %v, want one call for this provider with the new token", resolver.calls)
	}
}

// TestOAuthCallbackConnectsWithoutAProject is the fail-open half of the design:
// the operator consented and the tokens are valid, so a bootstrap that has no
// answer stores the credential anyway. Rejecting the login here would make one
// vendor-side lookup able to erase an authorization the vendor granted.
func TestOAuthCallbackConnectsWithoutAProject(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithBootstrap("gc"))
	fixture.service.projects = &fakeProjectResolver{}

	connected, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "gc", Code: "the-code", State: startFlow(t, fixture, "gc"),
	})
	if err != nil {
		t.Fatalf("Callback() error = %v, want the account landed without a project", err)
	}
	if got := connected.Endpoint.OAuth().ProjectID(); got != "" {
		t.Fatalf("stored project = %q, want empty", got)
	}
	if connected.Endpoint.OAuth().AccessTokenEncrypted() == "" {
		t.Fatal("the credential was not stored alongside the missing project")
	}
}

func TestOAuthCallbackSkipsAProviderThatDeclaresNoBootstrap(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("plain"))
	resolver := &fakeProjectResolver{project: "proj-9", ok: true}
	fixture.service.projects = resolver

	if _, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "plain", Code: "the-code", State: startFlow(t, fixture, "plain"),
	}); err != nil {
		t.Fatalf("Callback(): %v", err)
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("resolver calls = %v, want none for an entry that declares no bootstrap", resolver.calls)
	}
}

// TestOAuthCallbackWithoutAResolverConnects holds the wiring-optional rule: a
// deployment that never installs a resolver behaves exactly as it did before,
// rather than failing every login whose provider declares a bootstrap.
func TestOAuthCallbackWithoutAResolverConnects(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithBootstrap("gc"))

	connected, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "gc", Code: "the-code", State: startFlow(t, fixture, "gc"),
	})
	if err != nil {
		t.Fatalf("Callback() error = %v, want a connect that simply stores no project", err)
	}
	if connected.Endpoint.OAuth().ProjectID() != "" {
		t.Fatal("a flow without a resolver stored a project it never asked for")
	}
}
