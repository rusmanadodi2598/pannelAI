// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/console_buffer.go
// @for       The bounded console log ring: append, read, and clear.
// @uses      github.com/redis/go-redis/v9, context, time.
// @reason    SPEC-API-001 §7.13 exposes a console ring buffer of the last N log
//
//	lines. It lives in Redis rather than process memory because the
//	reference keeps it process-local and the panel polls it from a
//	management route; a Redis LPUSH+LTRIM is what makes the bound
//	hold across restarts and across more than one gateway process,
//	where an in-process slice would give each process its own history.
//
//	The bound is enforced by the trim in the same statement as the
//	append, so the list can never grow past max_records even briefly,
//	and the read is a bounded range rather than the whole list.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package redisrepo

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// consoleKey is the single list holding the console ring. One key is correct
// here because the buffer is global: the panel shows one console.
const consoleKey = "pannelai:console:lines"

// consoleCallTimeout bounds every Redis round trip so a hung server degrades the
// console read instead of stalling the request (AGENTS.md §1.6).
const consoleCallTimeout = 3 * time.Second

// ConsoleBuffer is the bounded log-line ring.
type ConsoleBuffer struct {
	client redis.UniversalClient
}

// NewConsoleBuffer constructs a Redis-backed console ring.
func NewConsoleBuffer(client redis.UniversalClient) *ConsoleBuffer {
	return &ConsoleBuffer{client: client}
}

// Append adds one line and trims the ring to maxRecords in a single pipeline,
// so the list is never observably longer than the bound. A non-positive
// maxRecords stores nothing rather than storing everything: if the configured
// bound is unusable, dropping the line is the failure that cannot grow without
// limit.
func (b *ConsoleBuffer) Append(ctx context.Context, line string, maxRecords int) error {
	if maxRecords < 1 || line == "" {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, consoleCallTimeout)
	defer cancel()

	pipe := b.client.Pipeline()
	pipe.LPush(callCtx, consoleKey, line)
	pipe.LTrim(callCtx, consoleKey, 0, int64(maxRecords-1))
	_, err := pipe.Exec(callCtx)
	return err
}

// Lines returns the ring's contents oldest first, at most maxRecords of them.
// Newest-first is how LPUSH stores them, so the range is reversed here: a
// console reads top to bottom, and making every caller reverse would be one
// reversal per caller.
func (b *ConsoleBuffer) Lines(ctx context.Context, maxRecords int) ([]string, error) {
	if maxRecords < 1 {
		return nil, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, consoleCallTimeout)
	defer cancel()

	lines, err := b.client.LRange(callCtx, consoleKey, 0, int64(maxRecords-1)).Result()
	if err != nil {
		return nil, err
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return lines, nil
}

// Clear empties the buffer, which the DELETE route does server-side so every
// panel sees the same empty console.
func (b *ConsoleBuffer) Clear(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, consoleCallTimeout)
	defer cancel()
	return b.client.Del(callCtx, consoleKey).Err()
}

// Size reports how many lines the ring currently holds, which a test asserts
// the eviction boundary against without reading the whole list.
func (b *ConsoleBuffer) Size(ctx context.Context) (int64, error) {
	callCtx, cancel := context.WithTimeout(ctx, consoleCallTimeout)
	defer cancel()
	return b.client.LLen(callCtx, consoleKey).Result()
}
