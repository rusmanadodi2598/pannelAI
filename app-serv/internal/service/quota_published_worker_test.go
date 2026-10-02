// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_published_worker_test.go
// @for       The fake clock, fake cache and fake provider read the published-quota poll worker is tested through.
// @uses      context, io, log/slog, sync, testing, time, internal/domain, internal/registry, internal/repository.
// @reason    Every rule this worker states is a decision about which calls were made and which rows written, so it is asserted against doubles that count them rather than a store and a network that would hide both.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-10-02
package service

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

var pollIndex = &fakeIndex{known: map[string]registry.Provider{"groq": usageEntry("groq", true), "claude": usageEntry("claude", true), "plain": {ID: "plain"}}}

// pollStore is the cache port as a counter; each case polls an endpoint once.
type pollStore struct {
	mu       sync.Mutex
	due      []domain.PublishedState
	attempts map[string]domain.PublishedAttempt
	cached   map[string]domain.PublishedQuota
}

func (s *pollStore) ListPublishedByEndpointIDs(_ context.Context, _ []string) (map[string]domain.PublishedQuota, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cached, nil
}
func (s *pollStore) DueForRefresh(_ context.Context, _ time.Time, limit int) ([]domain.PublishedState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.due[:min(limit, len(s.due))], nil
}
func (s *pollStore) RecordAttempt(_ context.Context, attempt domain.PublishedAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[attempt.EndpointID] = attempt
	return nil
}

// StorePublished stands in for the real prune: an answer replaces the endpoint's windows.
func (s *pollStore) StorePublished(_ context.Context, answer domain.PublishedAnswer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cached[answer.EndpointID] = domain.PublishedQuota{Windows: answer.Windows}
	return nil
}

// pollFetcher is the provider seam; calls and maxSeen are read after the sweep, so only the
// in-flight counter needs the lock.
type pollFetcher struct {
	mu       sync.Mutex
	replies  map[string]PublishedUsage
	errs     map[string]error
	hold     time.Duration
	calls    int
	inFlight int
	maxSeen  int
}

func (f *pollFetcher) fetch(_ context.Context, endpointID string) (PublishedUsage, error) {
	f.mu.Lock()
	f.calls++
	f.inFlight++
	f.maxSeen = max(f.maxSeen, f.inFlight)
	usage, err, hold := f.replies[endpointID], f.errs[endpointID], f.hold
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.inFlight--; f.mu.Unlock() }()
	time.Sleep(hold)
	return usage, err
}

type pollLister struct {
	mu       sync.Mutex
	accounts []domain.UpstreamEndpoint
	status   string
}

func (l *pollLister) List(_ context.Context, filter repository.EndpointFilter, q repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status = filter.Status
	start := (q.Page - 1) * q.PerPage
	if start >= len(l.accounts) {
		return nil, int64(len(l.accounts)), nil
	}
	return l.accounts[start:min(start+q.PerPage, len(l.accounts))], int64(len(l.accounts)), nil
}

type pollWorld struct {
	store   *pollStore
	lister  *pollLister
	fetcher *pollFetcher
	worker  *QuotaPublishedWorker
}

// newPollWorld wires the worker over the three doubles with a fixed clock and a seeded jitter. No
// quota service is wired: Fetch is the seam, so no credential and no network exist in these tests.
func newPollWorld(t *testing.T, budget, concurrency int, due []domain.PublishedState, accounts []domain.UpstreamEndpoint) *pollWorld {
	t.Helper()
	world := &pollWorld{store: &pollStore{due: due, attempts: map[string]domain.PublishedAttempt{}, cached: map[string]domain.PublishedQuota{}},
		lister: &pollLister{accounts: accounts}, fetcher: &pollFetcher{replies: map[string]PublishedUsage{}, errs: map[string]error{}}}
	worker, err := NewQuotaPublishedWorker(QuotaPublishedWorkerDeps{
		Store: world.store, Endpoints: world.lister, Providers: pollIndex, Fetch: world.fetcher.fetch,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Policy: PublishedPollPolicy{
			Budget: budget, Concurrency: concurrency, SeedPageSize: 3, FetchTimeout: 2 * time.Second, StoreTimeout: time.Second}})
	if err != nil {
		t.Fatalf("NewQuotaPublishedWorker() error = %v", err)
	}
	worker.clock, worker.jitter, world.worker = func() time.Time { return testNow }, newPublishedJitter(1), worker
	return world
}

// pollState builds one scheduling row; a negative gap is an endpoint nobody has ever asked.
func pollState(endpointID, providerID string, gapMinutes, failures int) domain.PublishedState {
	state := domain.PublishedState{EndpointID: endpointID, ProviderID: providerID,
		ConsecutiveFailures: failures, NextAttemptAt: testNow}
	if gapMinutes >= 0 {
		last := testNow.Add(-time.Duration(gapMinutes) * time.Minute)
		state.LastAttemptAt, state.FetchedAt = &last, &last
	}
	return state
}

func pollUsage(endpointID, providerID string) PublishedUsage {
	return PublishedUsage{EndpointID: endpointID, ProviderID: providerID, Plan: "Pro", FetchedAt: testNow,
		Windows: []PublishedWindow{{Label: "weekly", Used: 1, Total: 10, HasTotal: true}}}
}

func answerAll(world *pollWorld, due []domain.PublishedState) {
	for _, state := range due {
		world.fetcher.replies[state.EndpointID] = pollUsage(state.EndpointID, state.ProviderID)
	}
}

func pollAccount(t *testing.T, id, providerID string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, id+"@acct", domain.UpstreamAuthAPIKey, 1, testNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint(%s) error = %v", id, err)
	}
	return endpoint
}

func TestPublishedQuotaWorkerSweepPollsAtMostItsBudget(t *testing.T) {
	due := []domain.PublishedState{pollState("ep_1", "groq", -1, 0), pollState("ep_2", "groq", -1, 0), pollState("ep_3", "groq", -1, 0), pollState("ep_4", "groq", -1, 0)}
	world := newPollWorld(t, 2, 4, due, nil)
	answerAll(world, due)
	if polled := world.worker.Sweep(context.Background()); polled != 2 || world.fetcher.calls != 2 || len(world.store.attempts) != 2 {
		t.Fatalf("Sweep() polled %d, made %d calls, scheduled %d rows, want 2 of each", polled, world.fetcher.calls, len(world.store.attempts))
	}
}

func TestPublishedQuotaWorkerHoldsClaudeAtItsFamilyFloor(t *testing.T) {
	// Asked three minutes ago: inside claude's ten-minute floor, so a row that reads as due
	// anyway must not buy the account another call.
	world := newPollWorld(t, 40, 4, []domain.PublishedState{
		pollState("ep_claude", "claude", 3, 0), pollState("ep_groq", "groq", -1, 0)}, nil)
	world.fetcher.replies["ep_groq"] = pollUsage("ep_groq", "groq")
	last := testNow.Add(-3 * time.Minute)
	polled := world.worker.Sweep(context.Background())
	held := world.store.attempts["ep_claude"]
	if polled != 1 || world.fetcher.calls != 1 || !held.AttemptedAt.Equal(last) || !held.NextAttemptAt.Equal(last.Add(10*time.Minute)) {
		t.Fatalf("Sweep() polled %d with %d calls and re-armed claude to attempt %v next %v, want 1 call and the real attempt %v plus its floor",
			polled, world.fetcher.calls, held.AttemptedAt, held.NextAttemptAt, last)
	}
}

func TestPublishedQuotaWorkerBacksOffAFailurePastAFreshSuccess(t *testing.T) {
	world := newPollWorld(t, 40, 4, []domain.PublishedState{
		pollState("ep_good", "groq", -1, 0), pollState("ep_bad", "groq", 30, 4)}, nil)
	world.fetcher.replies["ep_good"] = pollUsage("ep_good", "groq")
	world.fetcher.errs["ep_bad"] = domain.NewInternalError("credential will not open")
	polled := world.worker.Sweep(context.Background())
	good, bad := world.store.attempts["ep_good"], world.store.attempts["ep_bad"]
	goodGap, badGap := good.NextAttemptAt.Sub(testNow), bad.NextAttemptAt.Sub(testNow)
	if polled != 1 || good.FailureDelta != 0 || bad.FailureDelta != 1 || badGap <= goodGap || badGap > 12*time.Minute {
		t.Fatalf("Sweep() polled %d with deltas %d and %d and waits %v against %v: want the failure backoff longer, inside the ceiling",
			polled, good.FailureDelta, bad.FailureDelta, badGap, goodGap)
	}
}

func TestPublishedQuotaWorkerKeepsStoredWindowsOnASoftAnswer(t *testing.T) {
	world := newPollWorld(t, 40, 4, []domain.PublishedState{pollState("ep_soft", "groq", 30, 0)}, nil)
	world.store.cached["ep_soft"] = domain.PublishedQuota{Windows: []domain.PublishedWindowRow{{Label: "weekly", Used: "7"}, {Label: "balance", Used: "3"}}}
	world.fetcher.replies["ep_soft"] = PublishedUsage{EndpointID: "ep_soft", ProviderID: "groq",
		FetchedAt: testNow, Message: "Usage API not implemented for groq"}
	world.worker.Sweep(context.Background())
	kept, soft := len(world.store.cached["ep_soft"].Windows), world.store.attempts["ep_soft"]
	if kept != 2 || soft.Message == nil || *soft.Message != "Usage API not implemented for groq" || soft.FailureDelta != 0 {
		t.Fatalf("soft answer left %d windows and scheduled %+v, want the 2 stored buckets kept and the sentence carried with no failure run", kept, soft)
	}
}

func TestPublishedQuotaWorkerCapsInFlightPolls(t *testing.T) {
	due := []domain.PublishedState{pollState("ep_1", "groq", -1, 0), pollState("ep_2", "groq", -1, 0), pollState("ep_3", "groq", -1, 0), pollState("ep_4", "groq", -1, 0),
		pollState("ep_5", "groq", -1, 0), pollState("ep_6", "groq", -1, 0), pollState("ep_7", "groq", -1, 0), pollState("ep_8", "groq", -1, 0)}
	world := newPollWorld(t, 40, 4, due, nil)
	world.fetcher.hold = 20 * time.Millisecond
	answerAll(world, due)
	polled := world.worker.Sweep(context.Background())
	if polled != 8 || world.fetcher.calls != 8 || world.fetcher.maxSeen > 4 || world.fetcher.maxSeen < 2 {
		t.Fatalf("polled %d with %d calls and %d in flight, want 8, 8, and the cap of 4 with real concurrency", polled, world.fetcher.calls, world.fetcher.maxSeen)
	}
}

// One slot in flight, so the jitter draws land in queue order and two sweeps of the same due set
// under the same seed must schedule the same instants.
func TestPublishedQuotaWorkerSweepIsDeterministicUnderASeededJitter(t *testing.T) {
	due := []domain.PublishedState{pollState("ep_1", "groq", -1, 0), pollState("ep_2", "groq", 30, 2), pollState("ep_3", "claude", -1, 0)}
	runs := make([]map[string]domain.PublishedAttempt, 0, 2)
	for run := 0; run < 2; run++ {
		world := newPollWorld(t, 40, 1, due, nil)
		answerAll(world, due)
		world.worker.Sweep(context.Background())
		runs = append(runs, world.store.attempts)
	}
	if !maps.Equal(runs[0], runs[1]) || runs[0]["ep_1"].NextAttemptAt.Equal(runs[0]["ep_2"].NextAttemptAt) {
		t.Fatalf("sweeps scheduled %+v against %+v: want identical rows, and a failure run further out than a fresh one",
			runs[0], runs[1])
	}
}

func TestPublishedQuotaWorkerSeedsEndpointsWithNoSchedule(t *testing.T) {
	world := newPollWorld(t, 40, 4, []domain.PublishedState{pollState("ep_late", "groq", 30, 0)},
		[]domain.UpstreamEndpoint{pollAccount(t, "ep_known", "groq"), pollAccount(t, "ep_new", "groq"),
			pollAccount(t, "ep_no_usage_api", "plain")})
	world.store.cached["ep_known"] = domain.PublishedQuota{Windows: []domain.PublishedWindowRow{{Label: "weekly", Used: "1"}}}
	armed, seeded := world.worker.Seed(context.Background()), world.store.attempts["ep_new"]
	if armed != 1 || !seeded.NextAttemptAt.Equal(testNow) || seeded.FailureDelta != 0 || seeded.Message != nil ||
		world.lister.status != string(domain.UpstreamEndpointActive) || world.worker.seedPage != 0 {
		t.Fatalf("Seed() armed %d and scheduled %+v, listed %q, cursor at %d: want only ep_new due now, no failure, no sentence, active accounts, wrapped walk",
			armed, seeded, world.lister.status, world.worker.seedPage)
	}
}
