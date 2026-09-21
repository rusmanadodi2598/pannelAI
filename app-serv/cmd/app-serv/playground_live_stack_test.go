//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/playground_live_stack_test.go
// @for       The live Playground gateway under test: the real router over the
//
//	real services, wired to PostgreSQL, Redis, and a local upstream.
//
// @uses      internal/dataplane, internal/domain, internal/handler,
// internal/provider, internal/registry, internal/repository/postgres,
// internal/repository/redis, internal/router, internal/schema,
// internal/service, context, os, testing, time.
// @reason    F9 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md requires
//
//	repeatable evidence from one request through the whole gateway, and the
//	evidence is only as good as the wiring behind it. This file builds that
//	wiring from the real constructors, so the tests read as scenarios
//	rather than as their own fixtures.
//
//	The file carries an `integration` build tag, so the default
//	`go test ./...` stays hermetic. With the tag active both DSNs are
//	required rather than optional: a silent skip would be exactly the
//	missing evidence this file exists to produce (AGENTS.md §2.1).
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration -run TestPlaygroundLive ./cmd/app-serv/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// The two environment variables this evidence needs. Both are required: a
// missing one fails the test rather than skipping it.
const (
	livePostgresEnv = "PANNELAI_TEST_POSTGRES_DSN"
	liveRedisEnv    = "PANNELAI_TEST_REDIS_ADDR"
)

// liveStack is the wired gateway under test: the real router over the real
// services, with PostgreSQL and Redis as the live dependencies.
type liveStack struct {
	mux   *router.Mux
	key   string
	pool  *pgxpool.Pool
	redis redis.UniversalClient
}

// newLiveStack builds the gateway against the live databases, seeds one active
// gateway key, and truncates the tables the evidence reads so the row counts
// belong to this run alone.
func newLiveStack(t *testing.T, upstream *liveUpstream) liveStack {
	t.Helper()
	dsn := os.Getenv(livePostgresEnv)
	if dsn == "" {
		t.Fatalf("%s must be set when running with -tags=integration", livePostgresEnv)
	}
	addr := os.Getenv(liveRedisEnv)
	if addr == "" {
		t.Fatalf("%s must be set when running with -tags=integration", liveRedisEnv)
	}
	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"usage_records", "request_logs", "gateway_keys", "upstream_endpoints"} {
		if _, err := pool.Exec(ctx, "TRUNCATE "+table+" CASCADE"); err != nil {
			t.Fatalf("truncating %s: %v", table, err)
		}
	}
	client := redis.NewClient(liveRedisOptions(addr))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushing Redis: %v", err)
	}

	key := domain.NewGatewayKey("playground-live", "sk-playground-live-secret", "sk-…live", time.Now().UTC())
	if err := postgres.NewGatewayKeyRepository(pool).Create(ctx, key); err != nil {
		t.Fatalf("seeding the gateway key: %v", err)
	}
	endpoint, err := domain.NewUpstreamEndpoint("ep-live", "live", "live endpoint", domain.UpstreamAuthNone, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("building the endpoint: %v", err)
	}
	if err := postgres.NewEndpointRepository(pool).Create(ctx, endpoint); err != nil {
		t.Fatalf("seeding the endpoint: %v", err)
	}

	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("building connectors: %v", err)
	}
	transport, err := dataplane.NewTransport(dataplane.TransportDeps{Connectors: connectors, Client: upstream.server.Client()})
	if err != nil {
		t.Fatalf("building the transport: %v", err)
	}
	resolver, err := dataplane.NewResolver(liveRegistry{provider: registry.Provider{
		ID: "live", AuthType: registry.AuthNone, NoAuth: true, PassthroughModels: true,
		Transport: registry.Transport{BaseURL: upstream.server.URL, Format: registry.DefaultFormat},
	}}, liveModelLookup{})
	if err != nil {
		t.Fatalf("building the resolver: %v", err)
	}
	selector, err := dataplane.NewSelector(dataplane.SelectorDeps{
		Endpoints:   postgres.NewEndpointRepository(pool),
		Cursor:      redisrepo.NewCursorStore(client),
		StickyLimit: 1,
	})
	if err != nil {
		t.Fatalf("building the selector: %v", err)
	}
	engine, err := dataplane.NewEngine(dataplane.EngineDeps{Resolver: resolver, Selector: selector, Transport: transport})
	if err != nil {
		t.Fatalf("building the engine: %v", err)
	}
	settingsRepo := postgres.NewSettingsRepository(pool)
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: settingsRepo})
	if err != nil {
		t.Fatalf("building the settings service: %v", err)
	}
	usageSvc, err := service.NewUsageService(service.UsageServiceDeps{
		Usage: postgres.NewUsageRepository(pool), Logs: postgres.NewLogRepository(pool), Settings: settings,
	})
	if err != nil {
		t.Fatalf("building the usage service: %v", err)
	}
	logSvc, err := service.NewLogService(service.LogServiceDeps{
		Logs: postgres.NewLogRepository(pool), Settings: settings, Console: redisrepo.NewConsoleBuffer(client),
	})
	if err != nil {
		t.Fatalf("building the log service: %v", err)
	}
	keyRepo := postgres.NewGatewayKeyRepository(pool)
	chat, err := service.NewChatService(service.ChatServiceDeps{
		Engine:    engine,
		Keys:      keyRepo,
		Settings:  settings,
		Usage:     usageSvc,
		Logs:      logSvc,
		Quotas:    service.NewQuotaCounter(redisrepo.NewQuotaCounterStore(client), nil),
		KeyUse:    keyRepo,
		RequestID: router.RequestIDFrom,
	})
	if err != nil {
		t.Fatalf("building the chat service: %v", err)
	}
	mux := router.New(router.Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "live", Commit: "live"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Chat:            handler.NewChatHandler(chat),
		RateLimiter:     redisrepo.NewLimiter(client),
		RateLimitPerMin: 60,
	})
	return liveStack{mux: mux, key: "sk-playground-live-secret", pool: pool, redis: client}
}

// TestPlaygroundLive_NonStreamedRequest is the F9 non-streamed evidence: one
// authenticated call through the whole gateway, with the status, the machine
// code, the request id, and the row counts it produced.
