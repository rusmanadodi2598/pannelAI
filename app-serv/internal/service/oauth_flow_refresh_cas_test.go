// Package service orchestrates the use cases the gateway exposes.
//
// @file      internal/service/oauth_flow_refresh_cas_test.go
// @for       The compare-and-swap an OAuth rotation writes through.
// @uses      context, testing, time, internal/domain
// @reason    A forced refresh and the worker's own sweep can both load one endpoint and both rotate
//
//	its token. Written as a plain UPDATE the slower one lands last and stores a token the vendor
//	has already replaced, which kills the account until the operator re-authenticates.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestOAuthRefresh_ConditionsTheWriteOnTheLoadedRow(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	seeded := seedOAuthEndpoint(t, fixture, "ep_cas", "identity-provider",
		"cas@example.com", "cas@example.com", testNow.Add(-time.Minute))

	if _, err := fixture.service.Refresh(context.Background(), "identity-provider", "ep_cas"); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if got := fixture.store.casLoadedAt; !got.Equal(seeded.UpdatedAt()) {
		t.Fatalf("write conditioned on %v, want the timestamp the row was loaded with (%v)",
			got, seeded.UpdatedAt())
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

func isConflict(err error) bool {
	return err != nil && domain.AsAppError(err).Code == "CONFLICT"
}
