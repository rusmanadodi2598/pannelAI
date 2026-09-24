//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_active_integration_test.go
// @for       The candidate-provider read behind `?active=true`: which providers
//
//	hold at least one endpoint the router would still pick.
//
// @uses      github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain, context, testing, time.
// @reason    The predicate is one SQL condition — status = 'active' — but it is
//
//	the same condition the data plane's candidates query narrows by,
//	and only a real server can prove the two agree over stored shapes
//	the in-memory fixtures cannot produce: an endpoint moved to error
//	by health tracking keeps a stale backoff timestamp, and a disabled
//	endpoint has its window cleared. Those rows are why the seam is a
//	distinct query rather than an approximation over the roll-up, and
//	this test stages all of them so the approximation can never creep
//	back in as "equivalent".
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-24
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// seedCandidateEndpoint stores one endpoint under the given provider with the
// given status, so a table can stage each state side by side.
func seedCandidateEndpoint(t *testing.T, repo *EndpointRepository, providerID, id string, status domain.UpstreamEndpointStatus, rateLimited bool, now time.Time) {
	t.Helper()
	// A no_auth endpoint needs no key, so the staged rows carry the state under
	// test and nothing else; the candidate predicate reads the endpoint table
	// alone either way.
	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, id, domain.UpstreamAuthNone, 1, now)
	if err != nil {
		t.Fatalf("building endpoint %s: %v", id, err)
	}
	switch status {
	case domain.UpstreamEndpointDisabled:
		if err := endpoint.Update(id, 1, "disabled", now); err != nil {
			t.Fatalf("disabling %s: %v", id, err)
		}
	case domain.UpstreamEndpointError:
		endpoint.MarkUnhealthy("staged for the test", now)
	}
	if rateLimited {
		endpoint.MarkRateLimited(now.Add(time.Minute))
	}
	if err := repo.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("storing %s: %v", id, err)
	}
}

// TestIntegration_ActiveProvidersMatchesTheCandidatesPredicate stages one
// provider per endpoint state — active, active-and-rate-limited, disabled,
// errored-with-a-stale-window — plus a provider asked about but absent from the
// table, and pins the one fact each of them must answer.
func TestIntegration_ActiveProvidersMatchesTheCandidatesPredicate(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	seedCandidateEndpoint(t, repo, "provider-active", "ep_active", domain.UpstreamEndpointActive, false, now)
	seedCandidateEndpoint(t, repo, "provider-backoff", "ep_backoff", domain.UpstreamEndpointActive, true, now)
	seedCandidateEndpoint(t, repo, "provider-disabled", "ep_disabled", domain.UpstreamEndpointDisabled, false, now)
	seedCandidateEndpoint(t, repo, "provider-error", "ep_error", domain.UpstreamEndpointError, true, now)

	asked := []string{
		"provider-active", "provider-backoff", "provider-disabled", "provider-error",
		"provider-absent",
	}
	got, err := repo.ActiveProviders(ctx, asked)
	if err != nil {
		t.Fatalf("ActiveProviders() error = %v", err)
	}

	cases := []struct {
		provider string
		want     bool
		why      string
	}{
		{"provider-active", true, "an active endpoint is a candidate"},
		{"provider-backoff", true, "an active endpoint in a backoff window is still a candidate: the router serves it when the window closes"},
		{"provider-disabled", false, "a disabled endpoint is a configuration an operator changed"},
		{"provider-error", false, "an errored endpoint is not a candidate even with a stale backoff timestamp left behind"},
		{"provider-absent", false, "a provider with no endpoint row holds no candidate"},
	}
	for _, tc := range cases {
		if got[tc.provider] != tc.want {
			t.Errorf("ActiveProviders()[%q] = %v, want %v (%s)",
				tc.provider, got[tc.provider], tc.want, tc.why)
		}
	}
}

// TestIntegration_ActiveProvidersEmptyInputIsANoRead pins the guard: an empty
// id list answers an empty map without reaching the database, which is the
// difference between "nothing to ask" and "ask about every provider".
func TestIntegration_ActiveProvidersEmptyInputIsANoRead(t *testing.T) {
	repo := newEndpointRepo(t)
	got, err := repo.ActiveProviders(context.Background(), nil)
	if err != nil {
		t.Fatalf("ActiveProviders(nil) error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ActiveProviders(nil) = %v, want an empty map", got)
	}
}
