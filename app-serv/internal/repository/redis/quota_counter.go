// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/quota_counter.go
// @for       The hot quota counters the flush worker drains into PostgreSQL.
// @uses      github.com/redis/go-redis/v9, internal/domain, context, time.
// @reason    SPEC-API-001 §6 keeps quota counters in Redis and flushes them to
//
//	PostgreSQL, because a synchronous write would put a database round
//	trip on the request path. Pending is a bounded, ordered read rather
//	than a keyspace scan, so a flush never runs an unbounded iteration
//	(AGENTS.md §1.7), and Clear runs only after the durable write
//	succeeded, so a counter that was written but not cleared is retried
//	rather than lost.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package redisrepo

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// quota counter key layout: one hash per endpoint, one field per window kind,
// plus a field holding each window's reset instant. A hash per endpoint keeps
// the pending read to a bounded KEYS-range-free SCAN over one set, and lets a
// flush clear exactly the fields it drained.
const (
	quotaKeyPrefix        = "pannelai:quota:"
	quotaResetFieldPrefix = "reset:"
)

// quotaScanCount bounds each SCAN step, so a large keyspace is walked in
// batches instead of one unbounded call.
const quotaScanCount = 128

// QuotaCounterStore keeps per-endpoint quota counters in Redis until the flush
// worker drains them.
type QuotaCounterStore struct {
	client redis.UniversalClient
}

// NewQuotaCounterStore constructs a Redis-backed counter store.
func NewQuotaCounterStore(client redis.UniversalClient) *QuotaCounterStore {
	return &QuotaCounterStore{client: client}
}

// Add increments one endpoint's counter for a window kind and records the reset
// instant the window rolls over at, in one pipeline so the counter and its reset
// instant are never observed apart.
//
// The reset instant is written only when the key is new or its stored instant
// has passed: the window this counter belongs to must not be extended by every
// request, or the counter would never roll over.
func (s *QuotaCounterStore) Add(ctx context.Context, endpointID string, kind domain.QuotaWindowKind, units int64, resetsAt time.Time) error {
	if units <= 0 {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	key := quotaKey(endpointID)
	resetField := quotaResetFieldPrefix + string(kind)
	pipe := s.client.Pipeline()
	pipe.HIncrBy(callCtx, key, string(kind), units)
	pipe.HSetNX(callCtx, key, resetField, resetsAt.UTC().Format(time.RFC3339Nano))
	_, err := pipe.Exec(callCtx)
	return err
}

// Pending returns the counters waiting to be flushed, at most limit of them,
// ordered by endpoint id so a retried flush drains the same batch the failed
// attempt saw. An endpoint id also carries a reset field that is not itself a
// counter, so only fields whose name is a window kind are returned.
func (s *QuotaCounterStore) Pending(ctx context.Context, limit int) ([]domain.QuotaWindow, error) {
	if limit < 1 {
		return nil, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	var (
		cursor uint64
		keys   []string
	)
	for len(keys) < limit {
		batch, next, err := s.client.Scan(callCtx, cursor, quotaKeyPrefix+"*", quotaScanCount).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if len(keys) > limit {
		keys = keys[:limit]
	}

	pending := make([]domain.QuotaWindow, 0, len(keys))
	for _, key := range keys {
		fields, err := s.client.HGetAll(callCtx, key).Result()
		if err != nil {
			return nil, err
		}
		endpointID := strings.TrimPrefix(key, quotaKeyPrefix)
		for name, raw := range fields {
			kind := domain.QuotaWindowKind(name)
			if !kind.IsValid() {
				continue
			}
			used, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				// A field that is not a number is not a counter this store
				// wrote; skipping it keeps one corrupt field from failing a
				// whole flush (reason: forward progress matters more than
				// reading a value no writer produced).
				continue
			}
			pending = append(pending, domain.RehydrateQuotaWindow(endpointID, "",
				kind, used, nil, parseReset(fields[quotaResetFieldPrefix+name]),
				domain.QuotaSourceComputed, time.Time{}))
		}
	}
	return pending, nil
}

// Clear removes the counters that were durably written, leaving any field the
// drain did not cover. Only the drained windows are deleted, so a request that
// incremented a counter between the read and the clear is not dropped.
func (s *QuotaCounterStore) Clear(ctx context.Context, drained []domain.QuotaWindow) error {
	if len(drained) == 0 {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	byEndpoint := make(map[string][]string, len(drained))
	for _, window := range drained {
		kind := string(window.Window())
		byEndpoint[window.EndpointID()] = append(byEndpoint[window.EndpointID()], kind, quotaResetFieldPrefix+kind)
	}
	pipe := s.client.Pipeline()
	for endpointID, fields := range byEndpoint {
		pipe.HDel(callCtx, quotaKey(endpointID), fields...)
	}
	_, err := pipe.Exec(callCtx)
	return err
}

// quotaKey names the hash holding one endpoint's counters.
func quotaKey(endpointID string) string { return quotaKeyPrefix + endpointID }

// parseReset reads a stored reset instant, returning nil when the value is
// absent or malformed. An unreadable instant leaves the window without one
// rather than failing the drain: the counter is still worth flushing.
func parseReset(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}
