//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_selection_stamp_integration_test.go
// @for       The selection stamp's own writes: a key, its account, and a row that vanished mid-request.
// @uses      context, testing, time, internal/domain
// @reason    The stamp is the only record of a credential being CHOSEN. An outcome write that carried the loaded value back would undo a newer choice made while the call ran, and an account with no key row has nothing else to record its turn. Split from the run-counting tests because the two sets of columns are written by two different moments.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-14
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestIntegration_MarkSelectedStampsTheKeyAndTheAccount(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	endpoint := seedHealthKey(t, repo, 0, now)
	if endpoint.Key(healthKeyID).LastSelectedAt() != nil {
		t.Fatal("a fresh key has not been chosen yet")
	}

	stamped := now.Add(2 * time.Second)
	if err := repo.MarkSelected(ctx, healthEndpointID, healthKeyID, stamped); err != nil {
		t.Fatalf("MarkSelected(key) error = %v", err)
	}
	stored := readHealthKey(t, repo, ctx)
	got := stored.Key(healthKeyID).LastSelectedAt()
	if got == nil || !got.Equal(stampPrecision(stamped)) {
		t.Fatalf("key stamp = %v, want %v", got, stampPrecision(stamped))
	}
	// Choosing a credential chose its account, and the store records both in the one
	// statement the domain describes: a panel that showed the key as spent while the
	// account still looked untouched would be telling two stories about one event.
	if account := stored.LastSelectedAt(); account == nil || !account.Equal(stampPrecision(stamped)) {
		t.Fatalf("account stamp = %v, want the same %v the key received", account, stampPrecision(stamped))
	}

	// An outcome write must not roll the stamp back to the value the caller loaded
	// before its call finished: a newer choice happened while this one was running.
	loaded, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	served, err := loaded.RecordKeySuccess(healthKeyID, stamped.Add(time.Second))
	if err != nil {
		t.Fatalf("RecordKeySuccess() error = %v", err)
	}
	if err := repo.RecordKeyHealth(ctx, served); err != nil {
		t.Fatalf("RecordKeyHealth() error = %v", err)
	}
	after := readHealthKey(t, repo, ctx).Key(healthKeyID).LastSelectedAt()
	if after == nil || !after.Equal(stampPrecision(stamped)) {
		t.Fatalf("key stamp after the outcome write = %v, want the selection's own %v",
			after, stampPrecision(stamped))
	}
}

// TestIntegration_MarkSelectedOnAKeylessAccount pins the OAuth path: an account with
// no key row carries the stamp on itself, because nothing else records its turn.
func TestIntegration_MarkSelectedOnAKeylessAccount(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	oauth, err := domain.NewUpstreamEndpoint("ep-oauth-rr", "provider-health", "oauth fixture",
		domain.UpstreamAuthOAuth, 1, now)
	if err != nil {
		t.Fatalf("building the endpoint: %v", err)
	}
	if err := repo.Create(ctx, oauth); err != nil {
		t.Fatalf("storing the account: %v", err)
	}

	stamped := now.Add(3 * time.Second)
	if err := repo.MarkSelected(ctx, "ep-oauth-rr", "", stamped); err != nil {
		t.Fatalf("MarkSelected(account) error = %v", err)
	}
	stored, err := repo.GetByID(ctx, "ep-oauth-rr")
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if got := stored.LastSelectedAt(); got == nil || !got.Equal(stampPrecision(stamped)) {
		t.Fatalf("account stamp = %v, want %v", got, stampPrecision(stamped))
	}
}

func TestIntegration_MarkSelectedLosesNothingWhenTheRowIsGone(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()

	// A key deleted between selection and stamp means the operator removed the
	// account mid-request. The call it announced still goes to the upstream that
	// already holds the credential, so a missing row is not the caller's problem.
	if err := repo.MarkSelected(ctx, "ep_missing", "key_missing", time.Now().UTC()); err != nil {
		t.Fatalf("MarkSelected(missing) error = %v, want nil", err)
	}
}

// stampPrecision drops a Go time to the microseconds PostgreSQL actually keeps. The column
// is `timestamptz`, six fractional digits, so a stamp from time.Now() comes back without its
// last three. Comparing at full precision would fail on a difference no reader can observe,
// and the ordering this feeds compares stored values that are all equally truncated.
func stampPrecision(at time.Time) time.Time {
	return at.Truncate(time.Microsecond)
}

// accountWithPark drives one keyless endpoint through the same transition a
// data-plane failure produces and persists it, then reads the row back.
func newKeylessAccount(t *testing.T, repo *EndpointRepository, at time.Time) domain.UpstreamEndpoint {
	t.Helper()
	ctx := context.Background()
	endpoint, err := domain.NewUpstreamEndpoint("ep-park", "provider-health", "park fixture",
		domain.UpstreamAuthOAuth, 1, at)
	if err != nil {
		t.Fatalf("building the account: %v", err)
	}
	if err := repo.Create(ctx, endpoint); err != nil {
		t.Fatalf("storing the account: %v", err)
	}
	return endpoint
}

// TestIntegration_AccountParkRoundTripsThroughTheOutcomeWrite is the persistence half
// of the account circuit: the domain parks, and this statement is what makes the park
// survive the next selector reading the row.
func TestIntegration_AccountParkRoundTripsThroughTheOutcomeWrite(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	at := time.Now().UTC()
	endpoint := newKeylessAccount(t, repo, at)

	if !endpoint.RecordAccountFailure(domain.KeyFailureAuth, domain.DefaultKeyHealthPolicy(), at) {
		t.Fatal("the account parked nothing")
	}
	if err := repo.RecordUpstreamOutcome(ctx, endpoint); err != nil {
		t.Fatalf("RecordUpstreamOutcome() error = %v", err)
	}

	stored, err := repo.GetByID(ctx, "ep-park")
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	parked := stored.RateLimitedUntil()
	if parked == nil || !parked.Equal(stampPrecision(*endpoint.RateLimitedUntil())) {
		t.Fatalf("account park = %v, want %v", parked, stampPrecision(*endpoint.RateLimitedUntil()))
	}
	if got := stored.AccountConsecutiveErrors(); got != 1 {
		t.Fatalf("account run = %d, want 1", got)
	}

	// The second strike counts on its own, in the store rather than from the copy
	// either caller happens to hold.
	reloaded, err := repo.GetByID(ctx, "ep-park")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	reloaded.RecordAccountFailure(domain.KeyFailureAuth, domain.DefaultKeyHealthPolicy(), at.Add(time.Minute))
	if err := repo.RecordUpstreamOutcome(ctx, reloaded); err != nil {
		t.Fatalf("second RecordUpstreamOutcome() error = %v", err)
	}
	if got := readAccount(t, repo, "ep-park").AccountConsecutiveErrors(); got != 2 {
		t.Fatalf("account run = %d, want 2", got)
	}

	// A served call releases both.
	served, err := repo.GetByID(ctx, "ep-park")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	served.RecordUpstreamSuccess(at.Add(2 * time.Hour))
	if err := repo.RecordUpstreamOutcome(ctx, served); err != nil {
		t.Fatalf("RecordUpstreamOutcome(success) error = %v", err)
	}
	released := readAccount(t, repo, "ep-park")
	if released.RateLimitedUntil() != nil {
		t.Fatalf("park = %v, want released by the served call", released.RateLimitedUntil())
	}
	if got := released.AccountConsecutiveErrors(); got != 0 {
		t.Fatalf("account run = %d, want 0", got)
	}
}

// TestIntegration_AccountParkNeverShortens is the merge rule kept on the endpoint
// column: a shorter deadline from a slower reader must not open the gate earlier than
// the longer one already stored.
func TestIntegration_AccountParkNeverShortens(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	at := time.Now().UTC()
	endpoint := newKeylessAccount(t, repo, at)

	long := domain.KeyHealthPolicy{AuthWindow: 2 * time.Hour}
	if !endpoint.RecordAccountFailure(domain.KeyFailureAuth, long, at) {
		t.Fatalf("the long strike parked nothing")
	}
	if err := repo.RecordUpstreamOutcome(ctx, endpoint); err != nil {
		t.Fatalf("storing the long park: %v", err)
	}
	// A second reader that saw the row before the first write landed proposes a
	// shorter deadline: transient parks flat, so its window is the one it sets.
	short := domain.KeyHealthPolicy{TransientWindow: 30 * time.Second}
	stale := readAccount(t, repo, "ep-park")
	if !stale.RecordAccountFailure(domain.KeyFailureTransient, short, at) {
		t.Fatalf("the short strike parked nothing")
	}
	if err := repo.RecordUpstreamOutcome(ctx, stale); err != nil {
		t.Fatalf("storing the short park: %v", err)
	}

	stored := readAccount(t, repo, "ep-park")
	// The short proposal was 30s from the same instant, so any deadline near the
	// hour proves the merge took the later one. The comparison is truncated because
	// the column stores microseconds; the margin is half an hour.
	parked := stored.RateLimitedUntil()
	if parked == nil || parked.Before(stampPrecision(at.Add(time.Hour))) {
		t.Fatalf("park = %v, want the longer deadline kept by GREATEST", parked)
	}
}

func readAccount(t *testing.T, repo *EndpointRepository, id string) domain.UpstreamEndpoint {
	t.Helper()
	stored, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("reading %s: %v", id, err)
	}
	return stored
}
