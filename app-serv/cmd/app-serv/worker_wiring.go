// Command app-serv wires the background workers to the process lifecycle.
//
// @file      cmd/app-serv/worker_wiring.go
// @for       Starts the quota flush, log retention, OAuth refresh, and published-quota poll workers on a context the shutdown path owns, each with a panic boundary and a joinable stop.
// @uses      internal/repository, internal/repository/postgres, internal/repository/redis, internal/service, context, fmt, log/slog, runtime/debug, time, github.com/redis/go-redis/v9.
// @reason    AGENTS.md §1.6 requires every goroutine to recover from a panic and to stop with the process, and SPEC-API-001 §6 gives all three workers their schedule. They live in one file because their shape is identical, construct, run until ctx is cancelled, supervise the panic, so a new worker has a pattern to follow rather than a new place to invent one.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-18
package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// oauthRefreshInterval is how often the worker sweeps for due tokens. It is
// well under the shortest refresh lead the registry declares, so a token that
// becomes due is refreshed on the next sweep rather than after its expiry.
const oauthRefreshInterval = 5 * time.Minute

// publishedPollTick is how often the published-quota worker walks its due queue. A
// minute is under the shortest family floor the poll policy declares (two minutes), so
// an endpoint that comes due is polled on the next tick rather than after a full
// interval, and the queue read, not the tick, is what bounds the provider calls.
const publishedPollTick = time.Minute

// publishedWorkerInputs is what only the published-quota poll worker needs: the quota
// service that owns the one live read, the cache it writes, the endpoint table it
// schedules new accounts from, and the index that says which families publish usage.
type publishedWorkerInputs struct {
	Quotas    *service.QuotaService
	Published repository.PublishedQuotaRepository
	Endpoints repository.EndpointRepository
	Index     service.ProviderIndex
	Policy    service.PublishedPollPolicy
}

// publishedWorker hands the poll worker from buildWorkers, which has its
// dependencies, to runWorkers, which has the boot context that terminates it.
//
// A package var rather than a managementDeps field because buildManagement does
// not receive that context, so the worker cannot be started there. Written once
// by buildWorkers and read once by runWorkers, both before any worker runs.
var publishedWorker *service.QuotaPublishedWorker

// buildWorkers constructs the workers that read state the management graph already
// built. The first two are returned rather than started here so the caller runs them
// only once the server is listening; the published-quota poll worker cannot be returned
// for that reason and is handed over instead, see publishedWorker.
func buildWorkers(
	client redis.UniversalClient,
	quotas *postgres.QuotaRepository,
	logs *service.LogService,
	published publishedWorkerInputs,
) (*service.QuotaFlusher, *service.LogRetentionWorker, error) {
	flusher, err := service.NewQuotaFlusher(
		redisrepo.NewQuotaCounterStore(client), quotas, service.DefaultQuotaFlushPolicy(), slog.Default(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("management wiring: quota flusher: %w", err)
	}
	// The retention worker shares the log service's purge, so the scheduled
	// deletion and the panel's purge route apply the same cutoff from the same
	// settings read.
	retention, err := service.NewLogRetentionWorker(logs, service.DefaultLogRetentionPolicy(), slog.Default())
	if err != nil {
		return nil, nil, fmt.Errorf("management wiring: log retention: %w", err)
	}
	// The poll worker is the only thing that fills the published-quota cache, so a
	// wiring gap here would leave the screen reading an empty cache forever rather
	// than fail loudly at boot.
	publishedWorker, err = service.NewQuotaPublishedWorker(service.QuotaPublishedWorkerDeps{
		Quotas: published.Quotas, Store: published.Published, Endpoints: published.Endpoints,
		Providers: published.Index, Policy: published.Policy, Logger: slog.Default(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("management wiring: published quota poll: %w", err)
	}
	return flusher, retention, nil
}

// workerGroup is the background half of the process. Its context is deliberately not
// the signal's: a worker stopped the moment SIGTERM arrives drains its queue while
// handlers are still running and still feeding it, so the events or counters those
// requests produce are recorded against a consumer that already left. The stop path
// ends the workers once the server has drained, and joins them.
type workerGroup struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// workerContext derives the context every background worker runs on. WithoutCancel is
// the whole point: the signal's context must not reach the workers, because the stop
// path chooses when they end, and it chooses after the server has drained.
func workerContext(signal context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(context.WithoutCancel(signal))
}

// runWorkers starts every background worker the management graph returned. A nil
// worker is a wiring gap the constructor already reported, so it is skipped here
// rather than crashed on at shutdown.
func runWorkers(parent context.Context, deps managementDeps) *workerGroup {
	group := &workerGroup{}
	group.ctx, group.cancel = workerContext(parent)

	if deps.QuotaFlusher != nil {
		group.start("quota flusher", func() { deps.QuotaFlusher.Run(group.ctx) })
	}
	if deps.LogRetention != nil {
		group.start("log retention", func() { deps.LogRetention.Run(group.ctx) })
	}
	if deps.OAuthRefresh != nil {
		group.start("oauth refresh", func() { deps.OAuthRefresh.Run(group.ctx, oauthRefreshInterval) })
	}
	// The usage event publisher drains the recorder's queue and the consumer
	// mirrors each event into the console ring. Both are supervised like every
	// other worker: the consumer owns a subscription that has to be closed on
	// shutdown, and the publisher owns the only goroutine that can publish.
	if deps.UsageEvents != nil {
		group.start("usage event publisher", func() { deps.UsageEvents.Run(group.ctx) })
	}
	if deps.UsageEventConsumer != nil {
		group.start("usage event consumer", func() { deps.UsageEventConsumer.Run(group.ctx) })
	}
	// The published-quota poll worker arrives through publishedWorker rather than
	// through deps, for the reason stated beside that var.
	if publishedWorker != nil {
		group.start("published quota poll", func() { publishedWorker.Run(group.ctx, publishedPollTick) })
	}
	return group
}

// start spawns one supervised worker and records it in the join.
func (g *workerGroup) start(name string, body func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		runSupervised(name, body)
	}()
}

// stopAndJoin ends the workers and waits for them to return, bounded by window.
//
// The wait has its own bound rather than trusting the workers: Run is written to
// return on ctx.Done(), and a worker that did not would otherwise hold the process
// open past what an orchestrator's kill timeout allows. Leaving one is logged, not
// hidden.
func (g *workerGroup) stopAndJoin(window time.Duration) {
	g.cancel()
	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(window):
		slog.Warn("workers did not finish inside the shutdown window", "window", window.String())
	}
}

// runSupervised runs one worker body with the §1.6 panic boundary. The worker's
// own Run returns when ctx is cancelled, which is the explicit termination
// condition; this wrapper only keeps a panic in one worker from killing the
// process the way an unrecovered goroutine would.
func runSupervised(name string, body func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("background worker panic recovered",
				"worker", name, "panic", recovered, "stack", string(debug.Stack()))
		}
	}()
	body()
}
