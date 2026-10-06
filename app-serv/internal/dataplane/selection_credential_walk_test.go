// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_credential_walk_test.go
// @for       The selection walk past one endpoint whose credential cannot be decrypted.
// @uses      context, errors, testing, time, internal/domain
// @reason    A rotated process key leaves stale ciphertext on individual rows. Aborting the walk on the first unreadable one made every healthy account of that provider unroutable, which is a provider outage caused by one bad row.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestSelector_SkipsAnEndpointWhoseCredentialIsUnreadable(t *testing.T) {
	stale := errors.New("the stored credential could not be decrypted")
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{
		relayEndpoint(t, "ep-1", "alpha"),
		relayEndpoint(t, "ep-2", "alpha"),
	}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo,
		Opener:    opener{failFor: map[string]error{"sealed-ep-1": stale}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection, err := selector.Select(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Select() error = %v, want the walk to continue to the healthy account", err)
	}
	if selection.Endpoint.ID() != "ep-2" {
		t.Fatalf("selected %q, want ep-2", selection.Endpoint.ID())
	}
	if credentialSecret(t, selection.Credential) != "plain-sealed-ep-2" {
		t.Fatalf("credential = %q, want the second account's own key", credentialSecret(t, selection.Credential))
	}
}

func TestSelector_ReportsTheCredentialErrorWhenNothingIsReadable(t *testing.T) {
	stale := errors.New("the stored credential could not be decrypted")
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{
		relayEndpoint(t, "ep-1", "alpha"),
		relayEndpoint(t, "ep-2", "alpha"),
	}
	selector, err := NewSelector(SelectorDeps{
		Endpoints: repo,
		Opener: opener{failFor: map[string]error{
			"sealed-ep-1": stale, "sealed-ep-2": stale,
		}},
	})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	selector.clock = func() time.Time { return now }

	if _, err := selector.Select(context.Background(), "alpha"); !errors.Is(err, stale) {
		t.Fatalf("Select() error = %v, want the credential failure surfaced rather than a bare no-provider", err)
	}
}
