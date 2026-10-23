// Package worker consumes queued work with bounded concurrency.
//
// @file      internal/worker/updates.go
// @for       The update stream loop: read a batch, own each entry, stop on cancel.
// @uses      context, log/slog, runtime/debug, time, repository/redis.
// @reason    Telegram work leaves the request path, so this consumer owns when an entry is acked: only after delivery, a dead letter, or a rejected payload, and one bad update never stops the loop.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-21
package worker

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
)

const (
	sendTimeout = 10 * time.Second
	dlqTimeout  = 5 * time.Second
)

// defaultRetryDelays is SPEC-BOT-001 §12.1: three attempts at 1 s, 4 s, 10 s.
var defaultRetryDelays = []time.Duration{time.Second, 4 * time.Second, 10 * time.Second}

// Queue is the consumer side of the update stream.
type Queue interface {
	EnsureGroup(ctx context.Context, stream, group string) error
	ReadUpdates(ctx context.Context, stream, group, consumer string, count int64, block time.Duration) ([]redis.Entry, error)
	AckUpdate(ctx context.Context, stream, group, id string) error
	EnqueueDLQ(ctx context.Context, payload []byte, reason string) error
}

// Sender delivers one message.
type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// permanentError marks a failure that must not be retried, such as a blocked
// user or a bad request. The sender classifies; the worker only obeys.
type permanentError interface {
	Permanent() bool
}

// Config names the stream, the consumer, and the retry shape.
type Config struct {
	Stream      string
	Group       string
	Consumer    string
	DLQStream   string
	Count       int64
	Block       time.Duration
	RetryDelays []time.Duration
}

// Consumer reads one batch at a time.
type Consumer struct {
	queue  Queue
	sender Sender
	logger *slog.Logger
	cfg    Config
}

// NewConsumer wires the stream, the sender, and the retry shape.
func NewConsumer(queue Queue, sender Sender, logger *slog.Logger, cfg Config) *Consumer {
	if cfg.Count <= 0 {
		cfg.Count = 10
	}
	if cfg.Block <= 0 {
		cfg.Block = 5 * time.Second
	}
	if len(cfg.RetryDelays) == 0 {
		cfg.RetryDelays = defaultRetryDelays
	}
	if cfg.DLQStream == "" {
		cfg.DLQStream = redis.DLQStream
	}
	return &Consumer{queue: queue, sender: sender, logger: logger, cfg: cfg}
}

// Run consumes until the context is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.queue.EnsureGroup(ctx, c.cfg.Stream, c.cfg.Group); err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		entries, err := c.queue.ReadUpdates(ctx, c.cfg.Stream, c.cfg.Group, c.cfg.Consumer, c.cfg.Count, c.cfg.Block)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Error("reading update stream failed", "error", err)
			if !sleep(ctx, time.Second) {
				return nil
			}
			continue
		}
		for _, entry := range entries {
			c.process(ctx, entry)
		}
	}
}

// process owns one entry end to end. A panic is contained here, not around the
// loop, so one update cannot end consumption for the whole process.
func (c *Consumer) process(ctx context.Context, entry redis.Entry) {
	defer func() {
		if recovered := recover(); recovered != nil {
			c.logger.Error("update worker panic recovered", "entry", entry.ID, "panic", recovered,
				"stack", string(debug.Stack()))
			c.deadLetter(ctx, entry, "panic")
		}
	}()

	item, err := decode(entry.Payload)
	if err != nil {
		c.logger.Error("decoding work item failed", "entry", entry.ID, "error", err)
		c.deadLetter(ctx, entry, "decode")
		return
	}
	if err := item.Validate(); err != nil {
		c.logger.Error("validating work item failed", "entry", entry.ID, "error", err)
		c.deadLetter(ctx, entry, "invalid")
		return
	}
	if err := c.deliver(ctx, item); err != nil {
		c.logger.Error("sending update failed", "entry", entry.ID, "error", err)
		c.deadLetter(ctx, entry, "send")
		return
	}

	ackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dlqTimeout)
	defer cancel()
	if err := c.queue.AckUpdate(ackCtx, c.cfg.Stream, c.cfg.Group, entry.ID); err != nil {
		c.logger.Error("acking update failed", "entry", entry.ID, "error", err)
	}
}
