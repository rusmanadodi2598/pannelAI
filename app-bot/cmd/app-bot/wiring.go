// Command app-bot runs the Telegram media bot service.
//
// @file      cmd/app-bot/wiring.go
// @for       Migration wiring, the HTTP timeouts, and the update consumers.
// @uses      context, fmt, log/slog, os, sync, time, internal/migrate, internal/migrations, pgxpool, repository/redis, telegram, worker.
// @reason    The migration set is embedded, so applying it is one call the root owns; the timeouts and the consumer pool are named here because a deployment tunes them.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-21
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
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
	updateDedupTTL = 7 * 24 * time.Hour
	workerGroup    = "bot-workers"
	// workerCount follows SPEC-BOT-001 §12.3: three goroutines, each handling
	// one update at a time.
	workerCount = 3
	workerBatch = 10
	workerBlock = 5 * time.Second
	// workerStopWindow covers one blocked read plus one in-flight send, which is
	// what "stop after the current item" costs at the configured timeouts.
	workerStopWindow = 15 * time.Second
)

// startWorker creates the consumer group before serving traffic, then runs one
// consumer per worker with its own name, so a pending entry always has exactly
// one identifiable owner. A consumer that dies returns on the failure channel
// instead of leaving readiness green over a dead loop.
func startWorker(logger *slog.Logger, queue *redisrepo.Client, sender *telegram.Client) (func(), <-chan error, error) {
	ctx, cancel := context.WithCancel(context.Background())
	if err := queue.EnsureGroup(ctx, redisrepo.UpdateStream, workerGroup); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("preparing the update stream: %w", err)
	}

	failures := make(chan error, workerCount)
	var group sync.WaitGroup

	for _, name := range consumerNames(workerCount) {
		consumer := worker.NewConsumer(queue, sender, logger, worker.Config{
			Stream:    redisrepo.UpdateStream,
			Group:     workerGroup,
			Consumer:  name,
			DLQStream: redisrepo.DLQStream,
			Count:     workerBatch,
			Block:     workerBlock,
		})
		group.Add(1)
		go func(c *worker.Consumer) {
			defer group.Done()
			if err := c.Run(ctx); err != nil && ctx.Err() == nil {
				failures <- fmt.Errorf("update consumer stopped: %w", err)
			}
		}(consumer)
	}

	stop := func() {
		cancel()
		done := make(chan struct{})
		go func() {
			group.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(workerStopWindow):
			logger.Warn("update workers did not stop in time")
		}
	}
	return stop, failures, nil
}

// consumerNames builds a per-instance consumer identity: a Redis pending list is
// only meaningful when its owners are distinguishable.
func consumerNames(count int) []string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "app-bot"
	}
	names := make([]string, 0, count)
	for index := 0; index < count; index++ {
		names = append(names, fmt.Sprintf("%s-%d-%d", host, os.Getpid(), index))
	}
	return names
}
