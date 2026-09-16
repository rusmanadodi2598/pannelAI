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
// @stability experimental
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
	"sync"
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

	// The logger is built after Config so LOG_LEVEL takes effect; Load has
	// already rejected an unsupported value.
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

	// Composition: repositories -> services -> handlers -> router.
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

	mux := router.New(router.Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   buildInfo(),
			Health: healthSvc,
		}),
		GatewayKey: handler.NewGatewayKeyHandler(keySvc),
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       300 * time.Second,
	}

	return serve(ctx, srv)
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
func buildInfo() schema.SystemInfo {
	return schema.SystemInfo{
		Version:          "0.1.0-dev",
		Commit:           "dev",
		BuildDate:        "dev",
		GoVersion:        "1.26",
		RegistryRevision: "none",
	}
}

// serve runs the HTTP server until it receives a termination signal, then
// shuts down within shutdownTimeout. Every goroutine here has an explicit
// termination condition (AGENTS.md §1.6).
func serve(ctx context.Context, srv *http.Server) error {
	serverErr := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("server goroutine panic recovered", "panic", r)
			}
		}()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	slog.Info("app-serv is listening", "addr", srv.Addr)

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-serverErr:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	wg.Wait()
	slog.Info("app-serv stopped cleanly")
	return nil
}

// redisPinger adapts go-redis to service.Pinger. The client's Ping returns a
// *StatusCmd rather than an error, so the adapter unwraps it; this keeps the
// service layer free of a Redis dependency (AGENTS.md §1.5).
type redisPinger struct {
	client *redis.Client
}

func (p redisPinger) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
