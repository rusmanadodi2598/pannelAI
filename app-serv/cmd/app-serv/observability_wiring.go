// Command app-serv wires the usage, quota, and log services.
//
// @file      cmd/app-serv/observability_wiring.go
// @for       Builds the usage, quota, and log services over the shared usage,
//
//	quota, and log repositories.
//
// @uses      internal/repository, internal/repository/redis, internal/service,
//
//	fmt, redis.
//
// @reason    The three services read the same repositories — the quota service
//
//	computes from the usage records, and the log service writes the rows
//	the usage detail route reads — so building them together keeps that
//	sharing visible. AGENTS.md §1.1 caps management_wiring.go, which is
//	the file that would otherwise carry this block.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// observability is the usage-side graph the boot sequence hands to the data
// plane and the router. It is a struct rather than a return list because the
// pieces are read at three different call sites, and a seven-value tuple makes
// transposing two of them a mistake the compiler cannot catch.
type observability struct {
	Usage     *service.UsageService
	Quota     *service.QuotaService
	Log       *service.LogService
	Publisher *service.UsageEventPublisher
	Consumer  *service.UsageEventConsumer
	Active    *service.ActiveRequestTracker
	UsageLive *service.UsageLiveService
}

// buildObservability assembles the usage, quota, and log services, and the
// usage event publisher and consumer that ride on the same Redis client
// (AGENTS.md §2.3: the recorder's choke point emits the event, the consumer is
// what makes it a seam rather than a decoration).
func buildObservability(
	usageRepo repository.UsageRecordRepository,
	quotaRepo repository.QuotaRepository,
	endpointRepo repository.EndpointRepository,
	logRepo repository.RequestLogRepository,
	settings *service.SettingsService,
	client redis.UniversalClient,
) (observability, error) {
	// The bus is built once and shared by both halves, so a publisher and a
	// subscriber cannot end up on different channels.
	bus := redisrepo.NewUsageEventBus(client)
	publisher := service.NewUsageEventPublisher(bus, slog.Default())

	usageSvc, err := service.NewUsageService(service.UsageServiceDeps{
		Usage: usageRepo, Logs: logRepo, Settings: settings, Events: publisher,
	})
	if err != nil {
		return observability{}, fmt.Errorf("management wiring: usage: %w", err)
	}

	// The endpoint repository is the quota service's existence seam: a cap is
	// only accepted for an endpoint the router could actually pick (draft 005
	// F2), and this is the same store every endpoint route reads.
	quotaSvc, err := service.NewQuotaService(service.QuotaServiceDeps{
		Quotas: quotaRepo, Usage: usageRepo, Endpoints: endpointRepo,
	})
	if err != nil {
		return observability{}, fmt.Errorf("management wiring: quotas: %w", err)
	}

	logSvc, err := service.NewLogService(service.LogServiceDeps{
		Logs: logRepo, Settings: settings, Console: redisrepo.NewConsoleBuffer(client),
	})
	if err != nil {
		return observability{}, fmt.Errorf("management wiring: logs: %w", err)
	}

	// The consumer mirrors each event into the console ring through the log
	// service, so the ring's bound and the settings read stay in one place.
	consumer := service.NewUsageEventConsumer(bus, logSvc, slog.Default())

	live, err := buildUsageLive(usageRepo, client)
	if err != nil {
		return observability{}, err
	}
	return observability{
		Usage: usageSvc, Quota: quotaSvc, Log: logSvc,
		Publisher: publisher, Consumer: consumer, Active: live.Active, UsageLive: live.Service,
	}, nil
}
