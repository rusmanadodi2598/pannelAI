// Package worker consumes queued work with bounded concurrency.
//
// @file      internal/worker/updates.go
// @for       The update stream consumer and its reply text.
// @uses      context, encoding/json, errors, fmt, log/slog, strings, time, schema.
// @reason    Telegram work leaves the request path, so one consumer owns retry semantics: an entry is acknowledged only after the send succeeds, and a poison payload is logged and dropped instead of blocking the stream.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-21
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

const sendTimeout = 10 * time.Second

// Queue is the consumer side of the update stream.
type Queue interface {
	EnsureGroup(ctx context.Context, stream, group string) error
	ReadUpdates(ctx context.Context, stream, group, consumer string, count int64, block time.Duration) ([]redis.Entry, error)
	AckUpdate(ctx context.Context, stream, group, id string) error
}

// Sender delivers one message.
type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// Config names the stream and the consumer.
type Config struct {
	Stream   string
	Group    string
	Consumer string
	Count    int64
	Block    time.Duration
}

// Consumer reads one batch at a time.
type Consumer struct {
	queue  Queue
	sender Sender
	logger *slog.Logger
	cfg    Config
}

// NewConsumer wires the stream, the sender, and the batch shape.
func NewConsumer(queue Queue, sender Sender, logger *slog.Logger, cfg Config) *Consumer {
	if cfg.Count <= 0 {
		cfg.Count = 10
	}
	if cfg.Block <= 0 {
		cfg.Block = 5 * time.Second
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
			if err := c.handle(ctx, entry); err != nil {
				c.logger.Error("handling update failed", "entry", entry.ID, "error", err)
				continue
			}
			if err := c.queue.AckUpdate(ctx, c.cfg.Stream, c.cfg.Group, entry.ID); err != nil {
				c.logger.Error("acking update failed", "entry", entry.ID, "error", err)
			}
		}
	}
}

func (c *Consumer) handle(ctx context.Context, entry redis.Entry) error {
	var item schema.WorkItem
	if err := json.Unmarshal(entry.Payload, &item); err != nil {
		return fmt.Errorf("decoding work item: %w", err)
	}
	if err := item.Validate(); err != nil {
		return fmt.Errorf("validating work item: %w", err)
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	return c.sender.SendMessage(sendCtx, item.ChatID, Reply(item))
}

// Reply is the P2 copy. It states what the bot does now and promises nothing
// that is not built yet.
func Reply(item schema.WorkItem) string {
	text := strings.TrimSpace(item.Text)
	switch {
	case text == "/start" || strings.HasPrefix(text, "/start "):
		return "Halo, ini KENTANG TECH Media.\n\nBot aktif dan pesanmu sudah diterima.\nGenerator image dan video masuk tahap berikutnya.\nKetik /help untuk daftar perintah."
	case text == "/help":
		return "Perintah yang aktif di fase ini:\n/start  mulai dan daftar akun\n/help   bantuan ini\n\nCredit, plan, dan generate menyusul di fase P3-P8."
	default:
		if item.Kind == schema.KindCallback {
			return "Tombol diterima. Aksi generate belum aktif di fase ini."
		}
		return "Pesan diterima. Generate image dan video belum aktif di fase ini."
	}
}

func sleep(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
