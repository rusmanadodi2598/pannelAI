// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_target_failure_test.go
// @for       Tests pinning that a target-shaped failure leaves every credential's health alone while the chain keeps walking.
// @uses      context, net/http, testing, internal/domain.
// @reason    Draft 050 T3 and T4: the class was derived from the status, not from the cause, so a model one vendor does not serve and a network fault inside the gateway both parked healthy credentials for two minutes and thirty seconds. These pins hold the boundary the router now keeps: the connection state is recorded, the next credential is still tried, and no key earns a window.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package dataplane

import (
	"context"
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// multiKeyRelayRepo builds one provider whose single account holds two keys, so
// a failure that empties the account is visible as a second credential being
// spent rather than as a provider that only ever had one.
func multiKeyRelayRepo(t *testing.T) *memEndpointRepo {
	t.Helper()
	repo := newMemEndpointRepo()
	endpoint, err := domain.NewUpstreamEndpoint("ep-multi", "alpha", "label-ep-multi", domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	for i, id := range []string{"uky-multi-a", "uky-multi-b"} {
		endpoint.AttachKey(domain.RehydrateUpstreamKey(
			id, "ep-multi", "key-"+id, "sealed-"+id, "…hint",
			i+1, domain.UpstreamKeyActive, nil, "", 0, domain.KeyRotationState{}, nil, now, now,
		))
	}
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{endpoint}
	return repo
}

// TestRelay_UnknownModelParksNoCredentialAcrossTheChain pins that a 404 walks
// every credential and parks none: another account may well serve the model, and
// the refusal says nothing about the keys that carried it.
func TestRelay_UnknownModelParksNoCredentialAcrossTheChain(t *testing.T) {
	var calls int
	server := alwaysRefusingUpstream(t, &calls, http.StatusNotFound)
	repo := multiKeyRelayRepo(t)
	engine := newRelayEngine(t, server.URL, repo, nil)

	_, err := engine.Relay(context.Background(), relayRequest("alpha/ghost"), nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want the upstream's refusal after both credentials")
	}
	if code := AsError(err).Code; code != CodeUpstreamError {
		t.Fatalf("code = %s, want %s for a 404 the upstream answered", code, CodeUpstreamError)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want 2: both credentials of the account are tried", calls)
	}
	if len(repo.health) != 0 {
		t.Fatalf("key health writes = %+v, want none for a refusal that is not about the credential", repo.health)
	}
	if len(repo.outcome) != 1 {
		t.Fatalf("endpoint parity writes = %d, want 1: the operator still sees the refusal", len(repo.outcome))
	}
}

// TestRelay_TransportFailureParksNoCredential pins that a call which never
// reached the upstream exercised no credential, so nothing is parked and the
// endpoint alone keeps the connection state.
func TestRelay_TransportFailureParksNoCredential(t *testing.T) {
	var calls int
	server := alwaysRefusingUpstream(t, &calls, http.StatusOK)
	repo := multiKeyRelayRepo(t)
	engine := newRelayEngine(t, server.URL, repo, nil)
	server.Close()

	if _, err := engine.Relay(context.Background(), relayRequest("alpha/works"), nil); err == nil {
		t.Fatal("Relay() = nil error, want the unreachable upstream to surface")
	}
	if len(repo.health) != 0 {
		t.Fatalf("key health writes = %+v, want none for a call the upstream never received", repo.health)
	}
	if len(repo.outcome) != 1 {
		t.Fatalf("endpoint parity writes = %d, want 1", len(repo.outcome))
	}
}

// TestRelay_UpstreamFiveKeepsParkingTheCredential is the boundary on the other
// side: an answered 5xx was carried by the credential, so it parks, and the
// endpoint-level window that would move it is not writable yet.
func TestRelay_UpstreamFiveKeepsParkingTheCredential(t *testing.T) {
	var calls int
	server := alwaysRefusingUpstream(t, &calls, http.StatusInternalServerError)
	repo := multiKeyRelayRepo(t)
	engine := newRelayEngine(t, server.URL, repo, nil)

	if _, err := engine.Relay(context.Background(), relayRequest("alpha/works"), nil); err == nil {
		t.Fatal("Relay() = nil error, want the upstream's failure to surface")
	}
	if len(repo.health) != 2 {
		t.Fatalf("key health writes = %d, want both credentials parked by the 5xx they carried", len(repo.health))
	}
	for id, key := range repo.health {
		if key.RateLimitedUntil() == nil {
			t.Fatalf("key %s has no window, want the 30s transient park", id)
		}
	}
}
