//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_key_health_integration_test.go
// @for       The counter arithmetic a key's health transition produces inside the store.
// @uses      context, testing, time, internal/domain
// @reason    Two selectors can load one key at the same count and each record a failure. Writing the value the aggregate computed would land one increment instead of two, and the circuit breaker that parks a bad credential would under-count exactly when a provider fails it hardest.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-04
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

const (
	healthEndpointID = "ep-key-health"
	healthKeyID      = "key-health-primary"
)

// seedHealthKey stores one endpoint holding one key whose circuit sits at the
// given error count, and returns the stored aggregate.
func seedHealthKey(t *testing.T, repo *EndpointRepository, failures int, now time.Time) domain.UpstreamEndpoint {
	t.Helper()
	ctx := context.Background()

	endpoint, err := domain.NewUpstreamEndpoint(
		healthEndpointID, "provider-health", "health fixture", domain.UpstreamAuthAPIKey, 1, now,
	)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}
	key, err := domain.NewUpstreamKey(
		"primary", healthEndpointID, healthKeyID, "sealed-ciphertext", "sk-****last", 1, now,
	)
	if err != nil {
		t.Fatalf("building key: %v", err)
	}
	if err := repo.Create(ctx, endpoint); err != nil {
		t.Fatalf("storing endpoint: %v", err)
	}
	if err := repo.AddKey(ctx, healthEndpointID, key); err != nil {
		t.Fatalf("storing key: %v", err)
	}
	if failures == 0 {
		stored, err := repo.GetByID(ctx, healthEndpointID)
		if err != nil {
			t.Fatalf("reading back: %v", err)
		}
		return stored
	}
	// Stage the starting count through the same path a data-plane call uses, so
	// the fixture is a real circuit state rather than a hand-written row.
	loaded, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	for range failures {
		updated, err := loaded.RecordKeyFailure(healthKeyID, "upstream refused", domain.KeyFailureAuth, domain.DefaultKeyHealthPolicy(), now)
		if err != nil {
			t.Fatalf("recording a failure: %v", err)
		}
		if err := repo.RecordKeyHealth(ctx, updated); err != nil {
			t.Fatalf("persisting a failure: %v", err)
		}
	}
	stored, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	return stored
}

func TestIntegration_RecordKeyHealthCountsEachConcurrentFailure(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Both selectors loaded the key at zero errors and each decided it had just
	// failed once, so each hands the store the same value: 1.
	loaded := seedHealthKey(t, repo, 0, now)
	failed, err := loaded.RecordKeyFailure(healthKeyID, "upstream refused", domain.KeyFailureAuth, domain.DefaultKeyHealthPolicy(), now)
	if err != nil {
		t.Fatalf("recording a failure: %v", err)
	}
	if got := failed.ConsecutiveErrors(); got != 1 {
		t.Fatalf("the aggregate reports %d errors, want the one failure it just recorded", got)
	}
	for range 2 {
		if err := repo.RecordKeyHealth(ctx, failed); err != nil {
			t.Fatalf("RecordKeyHealth() error = %v", err)
		}
	}

	after, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if got := after.Key(healthKeyID).ConsecutiveErrors(); got != 2 {
		t.Fatalf("consecutive_errors = %d, want 2: one of the two failures was overwritten", got)
	}
}

// TestIntegration_RecordKeyHealthNeverShortensAParkedWindow is the same race on
// the window column: two selectors compute their backoff from the count each
// read, so the shorter one must never release the longer park.
func TestIntegration_RecordKeyHealthNeverShortensAParkedWindow(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	seedHealthKey(t, repo, 0, now)
	long := healthKeyAt(now, now.Add(16*time.Second), 4)
	if err := repo.RecordKeyHealth(ctx, long); err != nil {
		t.Fatalf("RecordKeyHealth() the long window: %v", err)
	}
	stale := healthKeyAt(now, now.Add(2*time.Second), 1)
	if err := repo.RecordKeyHealth(ctx, stale); err != nil {
		t.Fatalf("RecordKeyHealth() the stale window: %v", err)
	}

	after := readHealthKey(t, repo, ctx)
	until := after.Key(healthKeyID).RateLimitedUntil()
	if until == nil {
		t.Fatal("rate_limited_until = nil, want the longer window to survive the stale write")
	}
	if got := until.Sub(now); got < 16*time.Second {
		t.Fatalf("park window = %v, want at least 16s: the 2s write shortened it", got)
	}
}

// TestIntegration_RecordKeyHealthSuccessStillClearsTheWindow is the half of the
// same statement that must NOT become monotonic: a served call is real evidence
// and releases the key, however long the window the previous failure parked.
func TestIntegration_RecordKeyHealthSuccessStillClearsTheWindow(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	seed := seedHealthKey(t, repo, 0, now)
	parked := healthKeyAt(now, now.Add(time.Minute), 3)
	if err := repo.RecordKeyHealth(ctx, parked); err != nil {
		t.Fatalf("RecordKeyHealth() the park: %v", err)
	}
	// The served write comes from the aggregate's own transition, not a hand
	// built row, because status is part of what this test asserts.
	served, err := seed.RecordKeySuccess(healthKeyID, now)
	if err != nil {
		t.Fatalf("recording a success: %v", err)
	}
	if err := repo.RecordKeyHealth(ctx, served); err != nil {
		t.Fatalf("RecordKeyHealth() the success: %v", err)
	}

	after := readHealthKey(t, repo, ctx)
	if until := after.Key(healthKeyID).RateLimitedUntil(); until != nil {
		t.Fatalf("rate_limited_until = %v, want a served call to clear the window", *until)
	}
	if got := after.Key(healthKeyID).Status(); got != domain.UpstreamKeyActive {
		t.Fatalf("status = %q, want active after a served call", got)
	}
	if got := after.Key(healthKeyID).ConsecutiveErrors(); got != 0 {
		t.Fatalf("consecutive_errors = %d, want 0 after a served call", got)
	}
}

// healthKeyAt builds the aggregate one selector would hand the store: the same
// key, its own window, and its own read of the count. A zero until means no
// window, which is what a success writes.
func healthKeyAt(now, until time.Time, failures int) domain.UpstreamKey {
	var window *time.Time
	if !until.IsZero() {
		edge := until
		window = &edge
	}
	return domain.RehydrateUpstreamKey(
		healthKeyID, healthEndpointID, "primary", "sealed-ciphertext", "sk-****last",
		1, domain.UpstreamKeyError, nil, "upstream refused", failures, window, now, now,
	)
}

func readHealthKey(t *testing.T, repo *EndpointRepository, ctx context.Context) domain.UpstreamEndpoint {
	t.Helper()
	after, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	return after
}

func TestIntegration_RecordKeyHealthClearIsNotAnIncrement(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	loaded := seedHealthKey(t, repo, 3, now)
	if got := loaded.Key(healthKeyID).ConsecutiveErrors(); got != 3 {
		t.Fatalf("staged circuit = %d errors, want 3", got)
	}
	cleared, err := loaded.RecordKeySuccess(healthKeyID, now)
	if err != nil {
		t.Fatalf("recording a success: %v", err)
	}
	// A served call lands twice (a racing success and a retried write): the
	// circuit stays clear rather than climbing out of the reset.
	for range 2 {
		if err := repo.RecordKeyHealth(ctx, cleared); err != nil {
			t.Fatalf("RecordKeyHealth() error = %v", err)
		}
	}

	after, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if got := after.Key(healthKeyID).ConsecutiveErrors(); got != 0 {
		t.Fatalf("consecutive_errors = %d after a clear, want 0", got)
	}
}
