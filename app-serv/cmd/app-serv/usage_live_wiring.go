// Command app-serv wires the live Usage surface.
//
// @file      cmd/app-serv/usage_live_wiring.go
// @for       Builds the in-flight marker store and the live Usage service over
//
//	the shared Redis client and usage repository.
//
// @uses      internal/repository, internal/repository/redis, internal/router,
//
//	internal/service, fmt, log/slog, redis.
//
// @reason    The marker store is written by the data plane and read by the §7.12
//
//	route, so one instance has to serve both halves. Building it here
//	keeps that single-instance rule visible, and keeps
//	observability_wiring.go inside its AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// usageLive is the pair the live Usage route needs: the tracker the data plane
// writes markers through, and the service the route reads them with.
type usageLive struct {
	Active  *service.ActiveRequestTracker
	Service *service.UsageLiveService
}

// buildUsageLive assembles the marker store, the tracker, and the live service.
func buildUsageLive(usageRepo repository.UsageRecordRepository, client redis.UniversalClient) (usageLive, error) {
	// One store instance is what makes the two halves agree: the tracker the data
	// plane writes through and the service the route reads with.
	activeStore := redisrepo.NewActiveRequestStore(client)
	liveSvc, err := service.NewUsageLiveService(service.UsageLiveServiceDeps{
		Active: activeStore, Usage: usageRepo,
	})
	if err != nil {
		return usageLive{}, fmt.Errorf("management wiring: usage live: %w", err)
	}
	return usageLive{
		Active:  service.NewActiveRequestTracker(activeStore, router.RequestIDFrom, slog.Default()),
		Service: liveSvc,
	}, nil
}
