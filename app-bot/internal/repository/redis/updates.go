// Package redis owns the Redis client.
//
// @file      internal/repository/redis/updates.go
// @for       The duplicate guard and the update stream.
// @uses      context, encoding/json, fmt, strconv, strings, time, redis, schema.
// @reason    The webhook must answer before any work happens, so the stream is the handoff and SET NX is the replay guard; both live here because a handler may not touch Redis directly.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-21
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	redisc "github.com/redis/go-redis/v9"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

// UpdateStream is the single work queue for Telegram updates.
const UpdateStream = "bot:stream:updates"

// DLQStream holds work the consumer could not complete, with its reason.
const DLQStream = "bot:dlq:updates"

const updateKeyPrefix = "bot:upd:"

// Entry is one stream message: the Redis id and its payload.
type Entry struct {
	ID      string
	Payload []byte
}

// MarkUpdate records the update id and reports whether it was new.
func (c *Client) MarkUpdate(ctx context.Context, updateID int64, ttl time.Duration) (bool, error) {
	key := updateKeyPrefix + strconv.FormatInt(updateID, 10)
	fresh, err := c.inner.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("marking update %d: %w", updateID, err)
	}
	return fresh, nil
}

// Mark and Forget are the service dedupe contract; the named variants below
// keep the Redis vocabulary visible at the call site.
func (c *Client) Mark(ctx context.Context, updateID int64, ttl time.Duration) (bool, error) {
	return c.MarkUpdate(ctx, updateID, ttl)
}

// Forget satisfies the service dedupe contract.
func (c *Client) Forget(ctx context.Context, updateID int64) error {
	return c.ForgetUpdate(ctx, updateID)
}

// ForgetUpdate releases a mark after a failed enqueue, so Telegram's retry can
// enter the queue instead of being dropped as a duplicate.
func (c *Client) ForgetUpdate(ctx context.Context, updateID int64) error {
	key := updateKeyPrefix + strconv.FormatInt(updateID, 10)
	if err := c.inner.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("releasing update %d: %w", updateID, err)
	}
	return nil
}

// Enqueue appends the work item to the stream.
//
// No MAXLEN: the stream is trimmed only once the dead letter path can hold what
// trimming would drop (SPEC-BOT-001 §9), and a lost entry here is a lost reply.
func (c *Client) Enqueue(ctx context.Context, item schema.WorkItem) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("encoding update %d: %w", item.UpdateID, err)
	}
	_, err = c.inner.XAdd(ctx, &redisc.XAddArgs{
		Stream: UpdateStream,
		// Values is a map because go-redis accepts only its own value shapes
		// here; this is the client boundary, not application data.
		Values: map[string]any{"payload": string(payload)},
	}).Result()
	if err != nil {
		return fmt.Errorf("appending update %d: %w", item.UpdateID, err)
	}
	return nil
}

// EnqueueDLQ records work the consumer could not finish, then it is safe to
// acknowledge the stream entry: the payload and the failure class survive.
func (c *Client) EnqueueDLQ(ctx context.Context, payload []byte, reason string) error {
	_, err := c.inner.XAdd(ctx, &redisc.XAddArgs{
		Stream: DLQStream,
		// Same client-boundary reason as Enqueue: go-redis wants a map of its
		// value shapes here.
		Values: map[string]any{
			"payload":   string(payload),
			"reason":    reason,
			"failed_at": time.Now().UTC().Format(time.RFC3339),
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("dead lettering update: %w", err)
	}
	return nil
}

// EnsureGroup creates the consumer group once. BUSYGROUP means it exists.
func (c *Client) EnsureGroup(ctx context.Context, stream, group string) error {
	err := c.inner.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("creating consumer group %s: %w", group, err)
	}
	return nil
}

// ReadUpdates reads one batch, blocking up to the given duration.
func (c *Client) ReadUpdates(ctx context.Context, stream, group, consumer string, count int64, block time.Duration) ([]Entry, error) {
	streams, err := c.inner.XReadGroup(ctx, &redisc.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if err == redisc.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading updates: %w", err)
	}

	entries := make([]Entry, 0, count)
	for _, streamResult := range streams {
		for _, message := range streamResult.Messages {
			payload, ok := message.Values["payload"].(string)
			if !ok {
				// Keep the entry so the consumer dead letters it. Skipping it
				// here would leave it in the pending list with no trace.
				entries = append(entries, Entry{ID: message.ID})
				continue
			}
			entries = append(entries, Entry{ID: message.ID, Payload: []byte(payload)})
		}
	}
	return entries, nil
}

// AckUpdate removes the entry from the pending list.
func (c *Client) AckUpdate(ctx context.Context, stream, group, id string) error {
	if err := c.inner.XAck(ctx, stream, group, id).Err(); err != nil {
		return fmt.Errorf("acking update %s: %w", id, err)
	}
	return nil
}
