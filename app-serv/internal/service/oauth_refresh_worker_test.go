// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_refresh_worker_test.go
// @for       Table-driven tests for the OAuth token refresh worker (P2, §6
//
//	of SYSTEM_MAP: exponential backoff with jitter, dead-letter marks
//	the endpoint error).
//
// @uses      context, sync, testing, time, internal/domain.
// @reason    AGENTS.md §1.6 makes a worker's retry policy and dead-letter
//
//	behaviour explicit requirements, so the table pins: a due token
//	is refreshed on the tick, a failing one is retried with a
//	growing delay, and after the stated attempts the endpoint is
//	marked error instead of retried forever. Cancellation ends the
//	run within one tick.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// workerFixture assembles the worker over the same fakes the flow tests use,
// with the clock under the test's control so backoff sleeps are simulated
// rather than waited for.
type workerFixture struct {
	worker  *OAuthRefreshWorker
	store   *memEndpointStore
	tokens  *fakeTokenClient
	index   *fakeIndex
	clockAt time.Time
	clockMu sync.Mutex
}

func newWorkerFixture(t *testing.T, providers ...registry.Provider) *workerFixture {
	t.Helper()
	index := &fakeIndex{known: map[string]registry.Provider{}}
	for _, provider := range providers {
		index.known[provider.ID] = provider
	}
	sealer, err := domain.NewSealer(oauthTestKey)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	fixture := &workerFixture{
		store:  newMemEndpointStore(),
		tokens: &fakeTokenClient{},
		index:  index,
	}
	svc, err := NewOAuthFlowService(OAuthFlowDeps{
		Index: index, Store: fixture.store, States: newFakeStateStore(),
		Tokens: fixture.tokens, Sealer: sealer,
	})
	if err != nil {
		t.Fatalf("NewOAuthFlowService: %v", err)
	}
	fixture.clockAt = testNow
	svc.clock = func() time.Time {
		fixture.clockMu.Lock()
		defer fixture.clockMu.Unlock()
		return fixture.clockAt
	}
	fixture.worker = NewOAuthRefreshWorker(svc, index)
	fixture.worker.sleep = func(time.Duration) {}
	return fixture
}

func (f *workerFixture) advance(d time.Duration) {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	f.clockAt = f.clockAt.Add(d)
}

func TestOAuthRefreshWorkerRefreshesDueTokens(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, oauthFlowFixture{store: fixture.store, sealer: mustWorkerSealer(t)},
		"ep_due", "identity-provider", "due@example.com", "due@example.com", testNow.Add(-time.Minute))
	seedOAuthEndpoint(t, oauthFlowFixture{store: fixture.store, sealer: mustWorkerSealer(t)},
		"ep_later", "identity-provider", "later@example.com", "later@example.com", testNow.Add(6*time.Hour))

	if refreshed := fixture.worker.Sweep(context.Background()); refreshed != 1 {
		t.Fatalf("refreshed = %d, want only the due account", refreshed)
	}
	endpoint, err := fixture.store.GetByID(context.Background(), "ep_due")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	opened, err := mustWorkerSealer(t).Open(endpoint.OAuth().AccessTokenEncrypted)
	if err != nil || opened != "at-issued" {
		t.Fatalf("token not refreshed: %q (%v)", opened, err)
	}
	later, err := fixture.store.GetByID(context.Background(), "ep_later")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if opened, _ := mustWorkerSealer(t).Open(later.OAuth().AccessTokenEncrypted); opened != "old-access" {
		t.Fatalf("not-due token was touched: %q", opened)
	}
}

func TestOAuthRefreshWorkerRetriesThenDeadLetters(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, oauthFlowFixture{store: fixture.store, sealer: mustWorkerSealer(t)},
		"ep_dead", "identity-provider", "dead@example.com", "dead@example.com", testNow.Add(-time.Minute))
	fixture.tokens.grantFn = func(TokenGrant) (TokenResponse, error) {
		return TokenResponse{}, domain.NewUpstreamError("the token endpoint refused the grant: invalid_grant")
	}

	// Attempts below the limit keep the endpoint active and count a retry.
	for attempt := 1; attempt < oauthRefreshMaxAttempts; attempt++ {
		if refreshed := fixture.worker.Sweep(context.Background()); refreshed != 0 {
			t.Fatalf("sweep %d refreshed = %d, want 0", attempt, refreshed)
		}
		fixture.advance(time.Hour)
		endpoint, err := fixture.store.GetByID(context.Background(), "ep_dead")
		if err != nil {
			t.Fatalf("reloading: %v", err)
		}
		if endpoint.Status() != domain.UpstreamEndpointActive {
			t.Fatalf("sweep %d status = %q, want active until attempts are exhausted", attempt, endpoint.Status())
		}
	}
	// The attempt that exhausts the policy dead-letters the endpoint.
	if refreshed := fixture.worker.Sweep(context.Background()); refreshed != 0 {
		t.Fatalf("final sweep refreshed = %d, want 0", refreshed)
	}
	endpoint, err := fixture.store.GetByID(context.Background(), "ep_dead")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if endpoint.Status() != domain.UpstreamEndpointError {
		t.Fatalf("status = %q, want error after %d attempts", endpoint.Status(), oauthRefreshMaxAttempts)
	}
	if endpoint.TestStatus().State != domain.EndpointTestFail {
		t.Fatalf("test state = %q, want fail with a reason", endpoint.TestStatus().State)
	}
	// A dead-lettered endpoint is not retried on later sweeps.
	fixture.advance(time.Hour)
	if refreshed := fixture.worker.Sweep(context.Background()); refreshed != 0 {
		t.Fatalf("post-dead-letter sweep refreshed = %d, want 0", refreshed)
	}
	if calls := len(fixture.tokens.grantCalls); calls != oauthRefreshMaxAttempts {
		t.Fatalf("grant calls = %d, want exactly the %d attempts", calls, oauthRefreshMaxAttempts)
	}
}

func TestOAuthRefreshWorkerBackoffGrowsPerAttempt(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, oauthFlowFixture{store: fixture.store, sealer: mustWorkerSealer(t)},
		"ep_retry", "identity-provider", "retry@example.com", "retry@example.com", testNow.Add(-time.Minute))
	fixture.tokens.grantFn = func(TokenGrant) (TokenResponse, error) {
		return TokenResponse{}, domain.NewUpstreamError("temporarily unavailable")
	}
	var slept []time.Duration
	fixture.worker.sleep = func(d time.Duration) { slept = append(slept, d) }

	fixture.worker.Sweep(context.Background())
	fixture.worker.Sweep(context.Background())
	if len(slept) != 2 {
		t.Fatalf("sleeps = %d, want one per failed attempt", len(slept))
	}
	if slept[1] <= slept[0] {
		t.Fatalf("backoff did not grow: %v then %v", slept[0], slept[1])
	}
	if slept[1] > oauthRefreshBackoffCeiling {
		t.Fatalf("backoff %v exceeds the ceiling %v", slept[1], oauthRefreshBackoffCeiling)
	}
}

func TestOAuthRefreshWorkerRunStopsWithContext(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		fixture.worker.Run(ctx, time.Millisecond)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

func mustWorkerSealer(t *testing.T) (sealer *domain.Sealer) {
	t.Helper()
	sealer, err := domain.NewSealer(oauthTestKey)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	return sealer
}
