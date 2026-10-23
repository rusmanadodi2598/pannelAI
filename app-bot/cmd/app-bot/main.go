// Command app-bot runs the Telegram media bot service.
//
// @file      cmd/app-bot/main.go
// @for       Process lifecycle: config, dependencies, HTTP server, shutdown.
// @uses      context, errors, flag, fmt, net/http, os, os/signal, syscall, internal packages.
// @reason    A composition root that only wires and runs keeps every decision testable in its own package, per AGENTS.md §1.5.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-21
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/postgres"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/service"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/telegram"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "app-bot: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	migrateOnly := flag.Bool("migrate", false, "apply pending migrations and exit")
	flag.Parse()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	logger := logx.New(cfg.LogLevel, cfg.AppEnv, os.Stdout)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, cfg.DBPoolMax)
	if err != nil {
		return err
	}
	defer pool.Close()

	if *migrateOnly {
		if err := applyMigrations(ctx, pool); err != nil {
			return err
		}
		logger.Info("migrations applied")
		return nil
	}

	rdb, err := redis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	botClient, err := telegram.NewClient(cfg.TelegramBotToken, cfg.TelegramAPIBase)
	if err != nil {
		return err
	}
	webhook := handler.NewWebhook(cfg.TelegramWebhookSecret, service.NewIntake(rdb, rdb, updateDedupTTL), logger)
	stopWorker, workerFailures, err := startWorker(logger, rdb, botClient)
	if err != nil {
		return err
	}
	defer stopWorker()

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: router.New(router.Deps{
			System:  handler.NewSystem(pool, rdb, cfg.ReadinessTimeout),
			Webhook: webhook,
			Logger:  logger,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				serverErr <- fmt.Errorf("http server panic: %v", recovered)
			}
		}()
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("serving HTTP: %w", err)
			return
		}
		serverErr <- nil
	}()

	logger.Info("listening", "addr", cfg.HTTPAddr)
	select {
	case err := <-serverErr:
		if err != nil {
			return err
		}
	case err := <-workerFailures:
		return err
	case <-ctx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down HTTP: %w", err)
	}
	logger.Info("stopped")
	return nil
}
