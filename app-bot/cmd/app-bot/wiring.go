// Command app-bot runs the Telegram media bot service.
//
// @file      cmd/app-bot/wiring.go
// @for       Migration wiring and the HTTP timeouts the server runs with.
// @uses      context, time, internal/migrate, internal/migrations, pgxpool.
// @reason    The migration set is embedded, so applying it is one call the root owns; the timeouts are named here because a deployment tunes them.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-21
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/migrate"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/telegram"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/worker"
	"github.com/rusmanadodi2598/pannelAI/app-bot/migrations"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	migrationTimeout  = 2 * time.Minute
)

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	list, err := migrate.Parse(migrations.FS, ".")
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()
	if err := migrate.Up(runCtx, pool, list); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}

const (
	updateDedupTTL   = 7 * 24 * time.Hour
	workerGroup      = "bot-workers"
	workerConsumer   = "worker-1"
	workerBatch      = 10
	workerBlock      = 5 * time.Second
	workerStopWindow = 5 * time.Second
)

// startWorker creates the consumer group before serving traffic, then runs the
// consumer until the returned stop function is called.
func startWorker(ctx context.Context, logger *slog.Logger, queue *redisrepo.Client, sender *telegram.Client) (func(), error) {
	if err := queue.EnsureGroup(ctx, redisrepo.UpdateStream, workerGroup); err != nil {
		return nil, fmt.Errorf("preparing the update stream: %w", err)
	}
	consumer := worker.NewConsumer(queue, sender, logger, worker.Config{
		Stream:   redisrepo.UpdateStream,
		Group:    workerGroup,
		Consumer: workerConsumer,
		Count:    workerBatch,
		Block:    workerBlock,
	})

	workerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("update worker panic recovered", "panic", recovered)
			}
		}()
		if err := consumer.Run(workerCtx); err != nil && workerCtx.Err() == nil {
			logger.Error("update worker stopped", "error", err)
		}
	}()

	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(workerStopWindow):
			logger.Warn("update worker did not stop in time")
		}
	}
	return stop, nil
}
