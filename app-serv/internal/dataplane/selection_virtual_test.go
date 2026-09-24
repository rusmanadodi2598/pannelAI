// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_virtual_test.go
// @for       The virtual endpoint a credential-free provider answers on when the
//
//	operator stored none.
//
// @uses      context, testing, internal/domain, internal/registry.
// @reason    Draft 029 §4.8 measured the gap: the reference injects a virtual
//
//	connection for every `noAuth` provider (src/sse/services/auth.js:45-63),
//	so a free lane works the moment its provider is listed, while this port
//	answered NO_PROVIDER_AVAILABLE until an operator invented an endpoint.
//	The rule is narrow on purpose, and these cases pin both directions: it
//	must fire for a credential-free provider with no row, and it must not
//	fire for a provider that needs a credential, for one that already has
//	rows, or when the registry is absent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// virtualRegistry answers the one provider lookup the rule makes.
type virtualRegistry map[string]registry.Provider

func (r virtualRegistry) Provider(name string) (registry.Provider, bool) {
	entry, ok := r[name]
	return entry, ok
}

// TestSelector_SynthesizesTheEndpointForAKeylessProvider pins the rule's firing
// direction: a provider that needs no credential and has no stored endpoint is
// served on a synthesized one, with no key material, so the request can proceed.
func TestSelector_SynthesizesTheEndpointForAKeylessProvider(t *testing.T) {
	cases := []struct {
		name  string
		entry registry.Provider
	}{
		{
			name:  "no_auth declared on the provider",
			entry: registry.Provider{ID: "free-a", NoAuth: true, Category: "free"},
		},
		{
			name: "no_auth declared on the transport",
			entry: registry.Provider{ID: "free-a", Category: "free",
				Transport: registry.Transport{NoAuth: true}},
		},
		{
			name: "auth type says none",
			entry: registry.Provider{ID: "free-a", Category: "free",
				AuthType: registry.AuthNone},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			selector, err := NewSelector(SelectorDeps{
				Endpoints: repo,
				Registry:  virtualRegistry{"free-a": tc.entry},
			})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selection, err := selector.Select(context.Background(), "free-a")
			if err != nil {
				t.Fatalf("Select() error = %v, want the virtual endpoint to be selected", err)
			}
			if !strings.HasPrefix(selection.Endpoint.ID(), VirtualEndpointIDPrefix) {
				t.Fatalf("endpoint id = %q, want the virtual prefix", selection.Endpoint.ID())
			}
			if selection.Endpoint.AuthType() != domain.UpstreamAuthNone {
				t.Fatalf("auth type = %q, want none", selection.Endpoint.AuthType())
			}
			if selection.Key.ID() != "" {
				t.Fatalf("key = %q, want no key material", selection.Key.ID())
			}
			if selection.Credential.HasCredential() {
				t.Fatal("the selection carries credential material, want none")
			}
			// A keyless selection must accept the health writes as no-ops rather
			// than failing the request on a bookkeeping error.
			if err := selector.RecordSuccess(context.Background(), selection); err != nil {
				t.Fatalf("RecordSuccess() error = %v, want a no-op", err)
			}
			if err := selector.RecordFailure(context.Background(), selection, "boom", domain.KeyFailureTransient); err != nil {
				t.Fatalf("RecordFailure() error = %v, want a no-op", err)
			}
		})
	}
}

// TestSelector_DoesNotInventAnEndpoint pins the rule's silent direction. Each
// case is a provider the rule must leave alone, because inventing an endpoint
// there would either guess at an account or shadow the operator's own row.
func TestSelector_DoesNotInventAnEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		entry    registry.Provider
		stored   bool
		registry bool
	}{
		{
			name:     "a provider that needs a key",
			entry:    registry.Provider{ID: "free-a", Category: "apikey", AuthType: registry.AuthAPIKey},
			registry: true,
		},
		{
			// The registry knows a different provider, so the rule must not
			// invent an entry for the id the request named.
			name:     "a provider the registry does not know",
			entry:    registry.Provider{ID: "other", NoAuth: true},
			registry: true,
		},
		{
			name:     "a provider with a stored endpoint already",
			entry:    registry.Provider{ID: "free-a", NoAuth: true},
			stored:   true,
			registry: true,
		},
		{
			name:     "no registry is wired",
			entry:    registry.Provider{ID: "free-a", NoAuth: true},
			registry: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			if tc.stored {
				repo.byProvider["free-a"] = []domain.UpstreamEndpoint{noAuthEndpoint(t, "ep_stored", 1)}
			}
			deps := SelectorDeps{Endpoints: repo}
			if tc.registry {
				// The registry answers exactly one id, so a case naming another
				// provider reads the unknown-provider path.
				deps.Registry = virtualRegistry{tc.entry.ID: tc.entry}
			}
			selector, err := NewSelector(deps)
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selection, err := selector.Select(context.Background(), "free-a")
			if tc.stored {
				if err != nil {
					t.Fatalf("Select() error = %v, want the stored endpoint", err)
				}
				if selection.Endpoint.ID() != "ep_stored" {
					t.Fatalf("endpoint = %q, want the operator's own row", selection.Endpoint.ID())
				}
				return
			}
			if err == nil {
				t.Fatalf("Select() = endpoint %q, want a refusal", selection.Endpoint.ID())
			}
			if code := AsError(err).Code; code != CodeNoProvider {
				t.Fatalf("Select() code = %q, want %q", code, CodeNoProvider)
			}
		})
	}
}

// TestSelector_TheStoredRowWinsOverTheVirtualOne pins the precedence directly: a
// credential-free provider with both a stored endpoint and the rule available
// must serve the stored one, because the operator's own configuration is the
// account they meant.
func TestSelector_TheStoredRowWinsOverTheVirtualOne(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["free-a"] = []domain.UpstreamEndpoint{noAuthEndpoint(t, "ep_stored", 1)}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo,
		Registry:  virtualRegistry{"free-a": {ID: "free-a", NoAuth: true}},
	})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selection, err := selector.Select(context.Background(), "free-a")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selection.Endpoint.ID() != "ep_stored" {
		t.Fatalf("endpoint = %q, want the stored row to win", selection.Endpoint.ID())
	}
}
