// Package service orchestrates the use cases the gateway exposes.
//
// @file      internal/service/oauth_flow_refresh_cas_test.go
// @for       The compare-and-swap an OAuth rotation writes through.
// @uses      context, testing, time, internal/domain
// @reason    A forced refresh and the worker's own sweep can both load one endpoint and both rotate its token. Written as a plain UPDATE the slower one lands last and stores a token the vendor has already replaced, which kills the account until the operator re-authenticates.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestOAuthRefresh_ConditionsTheWriteOnTheLoadedCredential(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	seeded := seedOAuthEndpoint(t, fixture, "ep_cas", "identity-provider",
		"cas@example.com", "cas@example.com", testNow.Add(-time.Minute))

	if _, err := fixture.service.Refresh(context.Background(), "identity-provider", "ep_cas"); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	want := seeded.OAuth()
	got := fixture.store.casLoadedCredential
	if got.AccessTokenEncrypted() != want.AccessTokenEncrypted() ||
		got.RefreshTokenEncrypted() != want.RefreshTokenEncrypted() {
		t.Fatal("the write was conditioned on a credential other than the one the row was loaded with")
	}
}

func TestOAuthRefresh_LosingRaceKeepsTheWinnerSToken(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, fixture, "ep_loser", "identity-provider",
		"loser@example.com", "loser@example.com", testNow.Add(-time.Minute))
	fixture.store.casReject = true

	if _, err := fixture.service.Refresh(context.Background(), "identity-provider", "ep_loser"); !isConflict(err) {
		t.Fatalf("Refresh() error = %v, want the conflict a lost compare-and-swap reports", err)
	}
	endpoint, err := fixture.store.GetByID(context.Background(), "ep_loser")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	opened, err := fixture.sealer.Open(endpoint.OAuth().AccessTokenEncrypted())
	if err != nil || opened != "old-access" {
		t.Fatalf("stored access token = %q (%v), want the seeded one: the loser must not overwrite the winner",
			opened, err)
	}
}

// TestOAuthRefresh_BulkSweepKeepsGoingPastOneLostRace is the batch's accounting: two
// accounts are due, one loses its compare-and-swap to a writer that got there first,
// and the account that needed nothing of the sort must still come back refreshed. A
// sweep that returns the first error discards the report of work already committed,
// so the operator is told nothing happened when one account did move.
func TestOAuthRefresh_BulkSweepKeepsGoingPastOneLostRace(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, fixture, "ep_clean", "identity-provider",
		"clean@example.com", "clean@example.com", testNow.Add(-time.Minute))
	seedOAuthEndpoint(t, fixture, "ep_race", "identity-provider",
		"race@example.com", "race@example.com", testNow.Add(-time.Minute))
	fixture.store.casRejectFor = map[string]bool{"ep_race": true}

	outcome, err := fixture.service.Refresh(context.Background(), "identity-provider", "")
	if err != nil {
		t.Fatalf("Refresh() bulk error = %v, want the sweep to finish and account for the one skip", err)
	}
	if outcome.Refreshed != 1 || len(outcome.EndpointIDs) != 1 || outcome.EndpointIDs[0] != "ep_clean" {
		t.Fatalf("Refreshed = %d (%v), want exactly ep_clean", outcome.Refreshed, outcome.EndpointIDs)
	}
	if len(outcome.Skipped) != 1 || outcome.Skipped[0].EndpointID != "ep_race" {
		t.Fatalf("Skipped = %+v, want one entry naming ep_race", outcome.Skipped)
	}
	if !strings.Contains(outcome.Skipped[0].Reason, "changed during this refresh") {
		t.Fatalf("skip reason = %q, want the server's own conflict message", outcome.Skipped[0].Reason)
	}
}

// TestOAuthRefresh_OneNamedAccountStillReportsItsConflict pins the half that must not
// change: refreshing one account the operator asked for by name returns the refusal,
// because that is the concrete reason a forced action was asked for.
func TestOAuthRefresh_OneNamedAccountStillReportsItsConflict(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, fixture, "ep_named", "identity-provider",
		"named@example.com", "named@example.com", testNow.Add(-time.Minute))
	fixture.store.casRejectFor = map[string]bool{"ep_named": true}

	outcome, err := fixture.service.Refresh(context.Background(), "identity-provider", "ep_named")
	if !isConflict(err) {
		t.Fatalf("Refresh() error = %v, want the conflict surfaced to the caller", err)
	}
	if len(outcome.Skipped) != 0 {
		t.Fatalf("Skipped = %+v, want none: a single named account reports its failure as an error", outcome.Skipped)
	}
}

func isConflict(err error) bool {
	return err != nil && domain.AsAppError(err).Code == "CONFLICT"
}
