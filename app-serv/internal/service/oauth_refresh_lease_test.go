// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_refresh_lease_test.go
// @for       The OAuth sweep's behaviour when another replica holds the cycle lease.
// @uses      context, testing.
// @reason    A denied lease must skip the sweep without touching the provider, which is the whole point of the multi-replica guard.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package service

import (
	"context"
	"testing"
	"time"
)

type denyingLease struct{}

func (denyingLease) Acquire(context.Context) (bool, error) { return false, nil }
func (denyingLease) Release(context.Context) error         { return nil }

func TestOAuthRefreshWorkerSkipsAHeldLease(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, oauthFlowFixture{store: fixture.store, sealer: mustWorkerSealer(t)},
		"ep_leased", "identity-provider", "leased@example.com", "leased@example.com", testNow.Add(-time.Minute))
	fixture.worker.UseLease(denyingLease{})

	if refreshed := fixture.worker.Sweep(context.Background()); refreshed != 0 {
		t.Fatalf("sweep refreshed = %d, want the held lease to skip it", refreshed)
	}
}
