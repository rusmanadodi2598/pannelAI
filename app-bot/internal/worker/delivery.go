// Package worker consumes queued work with bounded concurrency.
//
// @file      internal/worker/delivery.go
// @for       Delivery of one work item: retry, cancellation, and the dead letter hop.
// @uses      bytes, context, encoding/json, errors, fmt, time, repository/redis, schema.
// @reason    The retry shape and the dead letter write are the two decisions that keep a reply from being lost silently, so they sit in their own file and their own tests.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-22
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

// deliver retries a retryable failure and returns the last error once the
// attempts are spent. The send runs on a detached context so a shutdown lets
// the in-flight message finish instead of aborting it mid-request.
func (c *Consumer) deliver(ctx context.Context, item schema.WorkItem) error {
	attempts := len(c.cfg.RetryDelays) + 1
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			delay := c.cfg.RetryDelays[attempt-1]
			if asked := retryAfter(last); asked > delay {
				delay = asked
			}
			if !sleep(ctx, delay) {
				return fmt.Errorf("retry cancelled after %d attempts: %w", attempt, ctx.Err())
			}
		}
		sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sendTimeout)
		err := c.sender.SendMessage(sendCtx, item.ChatID, Reply(item))
		cancel()
		if err == nil {
			return nil
		}
		last = err
		if isPermanent(err) {
			return err
		}
	}
	return last
}

// deadLetter preserves the payload before acknowledging, so a failure is
// inspectable instead of silently lost.
func (c *Consumer) deadLetter(ctx context.Context, entry redis.Entry, reason string) {
	dlqCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dlqTimeout)
	defer cancel()
	if err := c.queue.EnqueueDLQ(dlqCtx, entry.Payload, reason); err != nil {
		c.logger.Error("dead lettering update failed", "entry", entry.ID, "error", err)
		return
	}
	if err := c.queue.AckUpdate(dlqCtx, c.cfg.Stream, c.cfg.Group, entry.ID); err != nil {
		c.logger.Error("acking dead lettered update failed", "entry", entry.ID, "error", err)
	}
}

// decode reads one work item strictly: an unknown field is a producer bug, not
// a value to ignore.
func decode(payload []byte) (schema.WorkItem, error) {
	var item schema.WorkItem
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&item); err != nil {
		return schema.WorkItem{}, fmt.Errorf("decoding work item: %w", err)
	}
	return item, nil
}

func isPermanent(err error) bool {
	var permanent permanentError
	return errors.As(err, &permanent) && permanent.Permanent()
}

// retryAfterError is how long the transport asked us to wait.
type retryAfterError interface {
	RetryAfter() time.Duration
}

// maxRetryAfter keeps a runaway header from stalling a worker for minutes.
const maxRetryAfter = 30 * time.Second

func retryAfter(err error) time.Duration {
	var asked retryAfterError
	if !errors.As(err, &asked) {
		return 0
	}
	wait := asked.RetryAfter()
	if wait <= 0 {
		return 0
	}
	if wait > maxRetryAfter {
		return maxRetryAfter
	}
	return wait
}

func sleep(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
