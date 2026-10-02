// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_published_worker.go
// @for       The background provider poll: sweep the due queue on a tick, ask each account's provider through the existing live read, cache the answer.
// @uses      context, log/slog, runtime/debug, sync, sync/atomic, time, internal/domain, internal/repository.
// @reason    The quota screen may not fetch while it is read — one provider call per account
//
//	is the N+1 AGENTS.md §1.7 blocks here — and quota_published_cache.go only
//	answers from a cache somebody fills. This is it; what one answer writes
//	lives in quota_published_store.go and its intervals in
//	quota_published_policy.go, split by reason to change.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-10-02
package service

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// publishedPollFetch asks one account's provider at this instant. It is a field rather than a
// direct call for the reason PublishedQuotaFetcher is one: a sweep is testable with no network
// and no database. Production binds it to (*QuotaService).livePublishedUsage, which already
// owns endpoint lookup, the refusals and the credential — this worker touches neither.
type publishedPollFetch func(ctx context.Context, endpointID string) (PublishedUsage, error)

// publishedEndpointSource is the one endpoint read the scheduler needs, narrow like EndpointFinder.
type publishedEndpointSource interface {
	List(ctx context.Context, filter repository.EndpointFilter, q repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error)
}

// QuotaPublishedWorkerDeps holds the collaborators; a wiring with no quota service must supply Fetch.
type QuotaPublishedWorkerDeps struct {
	Quotas    *QuotaService
	Store     repository.PublishedQuotaRepository
	Endpoints publishedEndpointSource
	Providers ProviderIndex
	Fetch     publishedPollFetch
	Policy    PublishedPollPolicy
	Logger    *slog.Logger
}

// QuotaPublishedWorker polls provider-published quota into the screen's cache.
type QuotaPublishedWorker struct {
	store     repository.PublishedQuotaRepository
	endpoints publishedEndpointSource
	providers ProviderIndex
	fetch     publishedPollFetch
	policy    PublishedPollPolicy
	logger    *slog.Logger
	clock     func() time.Time

	// jitter adds the spread to every delay; concurrent polls draw from one source.
	jitter *publishedJitter
	// seedPage walks the endpoint table one page per tick; Run is its only caller.
	seedPage int
	// running is the single-flight guard against a sweep stacking on its own tick.
	running atomic.Bool
}

// NewQuotaPublishedWorker validates deps and returns a ready worker.
func NewQuotaPublishedWorker(deps QuotaPublishedWorkerDeps) (*QuotaPublishedWorker, error) {
	if deps.Store == nil || deps.Endpoints == nil || deps.Providers == nil {
		return nil, domain.NewValidationError("published quota worker requires its cache, an endpoint source, and a provider index")
	}
	fetch := deps.Fetch
	if fetch == nil {
		if deps.Quotas == nil {
			return nil, domain.NewValidationError("published quota worker requires the quota service or a fetch seam")
		}
		fetch = deps.Quotas.livePublishedUsage
	}
	policy := deps.Policy
	if policy.Budget < 1 || policy.Concurrency < 1 || policy.SeedPageSize < 1 ||
		policy.FetchTimeout <= 0 || policy.StoreTimeout <= 0 {
		return nil, domain.NewValidationError("published quota poll policy is invalid")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &QuotaPublishedWorker{
		store: deps.Store, endpoints: deps.Endpoints, providers: deps.Providers,
		fetch: fetch, policy: policy, logger: logger, clock: time.Now,
		jitter: newPublishedJitter(time.Now().UnixNano()),
	}, nil
}

// Run seeds and sweeps on the interval until ctx is cancelled — this worker's explicit
// termination condition (§1.6). The loop's panic boundary is runSupervised.
func (w *QuotaPublishedWorker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scheduled, polled := w.Seed(ctx), w.Sweep(ctx)
			if scheduled+polled > 0 {
				w.logger.Info("published quota worker swept", "scheduled", scheduled, "polled", polled)
			}
		}
	}
}

// Sweep asks at most policy.Budget due endpoints and returns how many it polled.
func (w *QuotaPublishedWorker) Sweep(ctx context.Context) int {
	if !w.running.CompareAndSwap(false, true) {
		return 0
	}
	defer w.running.Store(false)
	now := w.clock()
	queueCtx, cancel := context.WithTimeout(ctx, w.policy.StoreTimeout)
	due, err := w.store.DueForRefresh(queueCtx, now, w.policy.Budget)
	cancel()
	if err != nil {
		w.logger.Warn("published quota worker could not read the due queue", "budget", w.policy.Budget, "error", err)
		return 0
	}
	return w.pollAll(ctx, due, now)
}

// pollAll polls the due set with bounded concurrency: a buffered channel is the semaphore, a
// wait-group is the termination condition, so at most policy.Concurrency calls are in flight
// and none outlives the sweep. Each goroutine recovers its own panic (§1.6).
func (w *QuotaPublishedWorker) pollAll(ctx context.Context, due []domain.PublishedState, now time.Time) int {
	var wg sync.WaitGroup
	var polled atomic.Int64
	slots := make(chan struct{}, w.policy.Concurrency)
	for _, state := range due {
		if ctx.Err() != nil {
			break
		}
		if w.insideFloor(state, now) {
			w.rearmFloor(ctx, state)
			continue
		}
		slots <- struct{}{}
		wg.Add(1)
		go func(state domain.PublishedState) {
			defer wg.Done()
			defer func() { <-slots }()
			defer func() {
				if recovered := recover(); recovered != nil {
					w.logger.Error("published quota poll panic recovered",
						"endpoint", state.EndpointID, "panic", recovered, "stack", string(debug.Stack()))
				}
			}()
			if w.pollOne(ctx, state, now) {
				polled.Add(1)
			}
		}(state)
	}
	wg.Wait()
	return int(polled.Load())
}

// pollOne asks one endpoint and routes the reply to the writer that fits it. A Go error is a
// refusal or a storage fault; a sentence with no buckets is neither.
func (w *QuotaPublishedWorker) pollOne(ctx context.Context, state domain.PublishedState, now time.Time) bool {
	fetchCtx, cancel := context.WithTimeout(ctx, w.policy.FetchTimeout)
	result, err := w.fetch(fetchCtx, state.EndpointID)
	cancel()
	providerID := publishedPollProvider(state, result)
	switch {
	case err != nil:
		w.logger.Warn("published quota poll failed", "endpoint", state.EndpointID,
			"provider", providerID, "failures", state.ConsecutiveFailures+1, "error", err)
		w.schedule(ctx, publishedAttempt(state, providerID, now, 1),
			publishedNextDelay(providerID, state.ConsecutiveFailures+1))
		return false
	case len(result.Windows) == 0 && result.Failed:
		// A refusal the reference would have raised: the sentence is still worth showing,
		// but the poll counts, so the next attempt waits for the backoff.
		w.logger.Warn("published quota provider refused the read", "endpoint", state.EndpointID,
			"provider", providerID, "failures", state.ConsecutiveFailures+1, "message", result.Message)
		w.storeSoftAnswer(ctx, state, providerID, now, result, 1)
		return false
	case len(result.Windows) == 0:
		w.storeSoftAnswer(ctx, state, providerID, now, result, 0)
		return true
	default:
		return w.storeAnswer(ctx, state, providerID, now, result)
	}
}
