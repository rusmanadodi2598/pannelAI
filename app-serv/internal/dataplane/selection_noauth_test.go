// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_noauth_test.go
// @for       The selection and health rules for a no_auth endpoint, which
//
//	presents no credential and therefore owns no key to pick.
//
// @uses      internal/domain, context, testing, time.
// @reason    G15 in the P2 register: Select demanded a usable key before it
//
//	consulted the endpoint's auth type, so a no_auth endpoint without keys
//	answered NO_PROVIDER_AVAILABLE while Available() reported it usable.
//	These tests pin the two halves of the fix: such an endpoint is
//	selected with no material, and the health write it produces is a
//	no-op instead of a not-found error that would fail the request.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// noAuthEndpoint builds a no_auth endpoint for the fixture's provider.
func noAuthEndpoint(t *testing.T, id string, priority int) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, "provider-a", "label-"+id, domain.UpstreamAuthNone, priority, now)
	if err != nil {
		t.Fatalf("building no_auth endpoint %s: %v", id, err)
	}
	return endpoint
}

// TestSelector_SelectsAKeylessNoAuthEndpoint pins the routing half of G15: the
// endpoint is selectable without a key, the selection carries no material, and
// both health outcomes are no-ops rather than a not-found failure.
func TestSelector_SelectsAKeylessNoAuthEndpoint(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{noAuthEndpoint(t, "ep_free", 1)}

	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("Select() error = %v, want a keyless no_auth endpoint to be selectable", err)
	}
	if selection.Endpoint.ID() != "ep_free" {
		t.Fatalf("endpoint = %q, want ep_free", selection.Endpoint.ID())
	}
	if selection.Key.ID() != "" {
		t.Fatalf("key = %q, want none for a no_auth endpoint", selection.Key.ID())
	}
	if selection.Credential.APIKey != "" || selection.Credential.AccessToken != "" {
		t.Fatalf("credential = %+v, want no material", selection.Credential)
	}

	if err := selector.RecordSuccess(context.Background(), selection); err != nil {
		t.Fatalf("RecordSuccess() error = %v, want a no-op", err)
	}
	if err := selector.RecordFailure(context.Background(), selection, "upstream rejected"); err != nil {
		t.Fatalf("RecordFailure() error = %v, want a no-op", err)
	}
	if len(repo.health) != 0 {
		t.Fatalf("health writes = %d, want none for a keyless selection", len(repo.health))
	}
}

// TestSelector_SkipsADisabledNoAuthEndpoint is the benign control: the auth
// type does not bypass the availability rule, so a disabled no_auth endpoint
// still yields NO_PROVIDER_AVAILABLE.
func TestSelector_SkipsADisabledNoAuthEndpoint(t *testing.T) {
	repo := newMemEndpointRepo()
	disabled := noAuthEndpoint(t, "ep_off", 1)
	if err := disabled.Update("label-ep_off", 1, string(domain.UpstreamEndpointDisabled), now); err != nil {
		t.Fatalf("disabling endpoint: %v", err)
	}
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{disabled}

	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	if _, err := selector.Select(context.Background(), "provider-a"); err == nil {
		t.Fatal("Select() error = nil, want NO_PROVIDER_AVAILABLE for a disabled endpoint")
	}
}
