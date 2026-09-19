// Command app-serv wires the background workers to the process lifecycle.
//
// @file      cmd/app-serv/worker_wiring.go
// @for       Starts the quota flush, log retention, and OAuth refresh workers,
//
//	each with a panic boundary and a termination condition.
//
// @uses      internal/repository/postgres, internal/repository/redis,
//
//	internal/service, context, fmt, log/slog, runtime/debug, time,
//	github.com/redis/go-redis/v9.
//
// @reason    AGENTS.md §1.6 requires every goroutine to recover from a panic and
//
//	to stop with the process, and SPEC-API-001 §6 gives all three
//	workers their schedule. They live in one file because their shape is
//	identical — construct, run until ctx is cancelled, supervise the
//	panic — so a new worker has a pattern to follow rather than a new
//	place to invent one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// oauthRefreshInterval is how often the worker sweeps for due tokens. It is
// well under the shortest refresh lead the registry declares, so a token that
// becomes due is refreshed on the next sweep rather than after its expiry.
const oauthRefreshInterval = 5 * time.Minute

// buildWorkers constructs the two workers that read state the management graph
// already built. They are returned rather than started here so the caller runs
// them only once the server is listening.
func buildWorkers(
	client redis.UniversalClient,
	quotas *postgres.QuotaRepository,
	logs *service.LogService,
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
	return flusher, retention, nil
}

// runWorkers starts every background worker the management graph returned. A nil
// worker is a wiring gap the constructor already reported, so it is skipped here
// rather than crashed on at shutdown.
func runWorkers(ctx context.Context, deps managementDeps) {
	if deps.QuotaFlusher != nil {
		go runSupervised("quota flusher", func() { deps.QuotaFlusher.Run(ctx) })
	}
	if deps.LogRetention != nil {
		go runSupervised("log retention", func() { deps.LogRetention.Run(ctx) })
	}
	if deps.OAuthRefresh != nil {
		go runSupervised("oauth refresh", func() { deps.OAuthRefresh.Run(ctx, oauthRefreshInterval) })
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
