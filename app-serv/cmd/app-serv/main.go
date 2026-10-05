// Command app-serv is the composition root for the pannelAI gateway API.
//
// @file      cmd/app-serv/main.go
// @for       Process bootstrap: config, dependencies, server lifecycle.
// @uses      internal/config, internal/handler, internal/repository/postgres,
//
//	internal/router, internal/service.
//
// @reason    AGENTS.md §1.5 makes this file wiring only, no logic: it
//
//	constructs the dependency graph, fails fast on invalid config,
//	and shuts down gracefully on termination. The pool limits are set
//	here rather than left at library defaults (AGENTS.md §1.7).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-16
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// shutdownTimeout bounds graceful shutdown so a stuck connection does not hang
// termination indefinitely.
const shutdownTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatalf("app-serv: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// The logger is built after Config so LOG_LEVEL takes effect; Load has already rejected an unsupported value.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(cfg.LogLevel),
	})))
	slog.Info("configuration loaded", "env", cfg.AppEnv, "addr", cfg.HTTPAddr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := buildPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer func() { _ = rdb.Close() }()

	// The schema ships inside the binary, so a fresh checkout boots against an
	// empty database instead of failing at the first request (SPEC-API-001 §10).
	if err := migrations.Apply(ctx, cfg.PostgresDSN); err != nil {
		return err
	}

	// The egress policy is built before the provider connectors so the Qoder
	// exchange rides the guarded client (draft 042 R07); the trio travels with
	// the management graph.
	settingsSvc, egressPolicy, sealer, err := buildFoundation(cfg, pool)
	if err != nil {
		return err
	}
	// The provider registry and plugin seam are installed before anything can
	// serve a request, so no caller ever sees an unpopulated lookup.
	index, connectors, err := buildProviderRuntime(egressPolicy.Client)
	if err != nil {
		return err
	}

	// Composition: repositories -> services -> handlers -> router.
	authHandler, rateLimiter, err := buildAuth(ctx, cfg, pool, rdb)
	if err != nil {
		return err
	}
	keyRepo := postgres.NewGatewayKeyRepository(pool)
	keySvc, err := service.NewGatewayKeyService(service.GatewayKeyServiceDeps{
		Repo:   keyRepo,
		Prefix: cfg.GatewayKeyPrefix,
	})
	if err != nil {
		return err
	}
	healthSvc := service.NewHealthService(service.HealthServiceDeps{
		Postgres: pool,
		Redis:    redisPinger{client: rdb},
	})

	// The P1 management graph is built in its own file because a dozen services
	// would blow this file's line budget and mix the boot sequence with the graph.
	mgmt, err := buildManagement(cfg, pool, rdb, index, connectors, settingsSvc, egressPolicy, sealer, keyRepo)
	if err != nil {
		return err
	}

	// The registry revision travels with the index, so /version reports the document this process actually loaded.
	deps := routerDeps(cfg, authHandler, handler.NewGatewayKeyHandler(keySvc), healthSvc, rateLimiter, mgmt, index.Revision())
	if err := deps.AssertWired(); err != nil {
		return err
	}
	mux := router.New(deps)

	// The background workers run alongside the server and stop with the context, so shutdown leaves nothing running (AGENTS.md §1.6).
	runWorkers(ctx, mgmt)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       300 * time.Second,
		// Bound explicitly rather than left at Go's 1 MiB default: the request id
		// middleware echoes one header back to the client and into the access log,
		// and a megabyte of header per request is enough to make either expensive.
		MaxHeaderBytes: 64 << 10,
	}

	return serve(ctx, srv, quotaDrain(mgmt.QuotaFlusher))
}

// logLevel maps the validated configuration value to a slog level. Config has
// already rejected anything outside the accepted set, so the default branch is
// unreachable and exists only to keep the mapping total.
func logLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// buildPool builds the pgx pool with explicit limits (AGENTS.md §1.7).
func buildPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.PostgresDSN)
	if err != nil {
		return nil, errors.New("config: invalid POSTGRES_DSN")
	}
	pcfg.MaxConns = int32(cfg.DBPoolMax)
	pcfg.MinConns = 1
	pcfg.MaxConnLifetime = 30 * time.Minute
	pcfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return nil, errors.New("config: postgres is unreachable")
	}
	slog.Info("postgres pool ready", "max_conns", cfg.DBPoolMax)
	return pool, nil
}

// buildInfo reports the version endpoints payload (SPEC-API-001 §7.1).
//
// The registry revision is the document's own value, not a placeholder: §7.1
// exposes it so an operator can tell which registry a running gateway loaded,
// and a constant answers that question wrongly.
func buildInfo(registryRevision string) schema.SystemInfo {
	return schema.SystemInfo{
		Version:          "0.1.0-dev",
		Commit:           "dev",
		BuildDate:        "dev",
		GoVersion:        "1.26",
		RegistryRevision: registryRevision,
	}
}

// serve lives in shutdown_drain.go with the counter drain that has to run after
// the last request, because the two are one lifecycle.

// redisPinger adapts go-redis to service.Pinger. The client's Ping returns a
// *StatusCmd rather than an error, so the adapter unwraps it; this keeps the
// service layer free of a Redis dependency (AGENTS.md §1.5).
type redisPinger struct {
	client *redis.Client
}

func (p redisPinger) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
