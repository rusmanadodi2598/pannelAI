// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/quota_counter.go
// @for       The hot quota counter the flush worker mirrors into PostgreSQL.
// @uses      github.com/redis/go-redis/v9, internal/domain, context, time.
// @reason    SPEC-API-001 §6 keeps quota counters in Redis and flushes them to
//
//	PostgreSQL, because a synchronous write would put a database round
//	trip on the request path. The counter holds the window's RUNNING
//	TOTAL, not a per-batch delta: the flush mirrors that total and a
//	retried batch writes the same number twice without double-counting,
//	while the total survives the flush so the next tick's requests
//	continue from it instead of restarting the window. The per-batch
//	shape shipped first and was measured wrong live on 2026-09-23: two
//	requests of 65 and 68 tokens in different ticks left used_units at
//	68.
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

// quota counter key layout: one hash per endpoint, one field per window kind
// holding the running total, a reset field holding the instant the window rolls
// over at, and a flushed field holding the total the last durable write
// mirrored. A hash per endpoint keeps the pending read to a bounded SCAN over
// one set rather than a keyspace walk.
const (
	quotaKeyPrefix          = "pannelai:quota:"
	quotaResetFieldPrefix   = "reset:"
	quotaFlushedFieldPrefix = "flushed:"
)

// quotaScanCount bounds each SCAN step, so a large keyspace is walked in
// batches instead of one unbounded call.
const quotaScanCount = 128

// QuotaCounterStore keeps per-endpoint running totals in Redis until the flush
// worker has mirrored them into PostgreSQL.
type QuotaCounterStore struct {
	client redis.UniversalClient
}

// NewQuotaCounterStore constructs a Redis-backed counter store.
func NewQuotaCounterStore(client redis.UniversalClient) *QuotaCounterStore {
	return &QuotaCounterStore{client: client}
}

// Add advances one endpoint's running total for a window kind, restarting the
// window when its reset instant has passed. The rollover and the increment are
// one script (quota_counter_script.go), so a reader never observes a total
// carried into a closed window, and a request landing between the rollover
// check and the increment cannot bill the old window.
//
// The stored reset instant is written only when the window restarts: the reset
// the counter belongs to must not be extended by every request, or the window
// would never roll over.
func (s *QuotaCounterStore) Add(ctx context.Context, endpointID string, kind domain.QuotaWindowKind, units int64, resetsAt time.Time) error {
	if units <= 0 {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	if err := addScript.Run(callCtx, s.client,
		[]string{quotaKey(endpointID)},
		string(kind), quotaResetFieldPrefix+string(kind), quotaFlushedFieldPrefix+string(kind),
		units, resetsAt.UTC().UnixMicro()).Err(); err != nil {
		return err
	}
	return nil
}

// Pending returns the windows whose running total the flush has not mirrored
// yet, at most limit of them. A window already mirrored and still open is left
// out: rewriting an unchanged number every tick would churn the table for no
// information. A closed window is still returned, so the settle that retires it
// runs after a durable write of its final total.
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

	now := time.Now().UTC()
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
			resetsAt := parseReset(fields[quotaResetFieldPrefix+name])
			if fields[quotaFlushedFieldPrefix+name] == raw && resetsAt != nil && resetsAt.After(now) {
				continue
			}
			pending = append(pending, domain.RehydrateQuotaWindow(endpointID, "",
				kind, used, nil, resetsAt, domain.QuotaSourceComputed, time.Time{}))
		}
	}
	return pending, nil
}

// Settle records what a flush has made durable. A window whose total no request
// has changed since the flush is marked as mirrored; a window that has also
// closed is retired, because nothing would read it again and the next request
// restarts the window anyway. A window that changed in between keeps its
// counter and is written again next tick: those units are not durable yet.
//
// The comparison runs as one script per endpoint (quota_counter_script.go), so
// a request landing between the comparison and the write cannot lose its units.
func (s *QuotaCounterStore) Settle(ctx context.Context, drained []domain.QuotaWindow) error {
	if len(drained) == 0 {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	// One flat argument list per endpoint: window count, then the flushed-field
	// prefix, then one triple (counter field, reset field, mirrored total) per
	// window. The driver's script arguments are the one typed boundary to the
	// Lua script (AGENTS.md §1.4).
	byEndpoint := make(map[string][]interface{}, len(drained))
	for _, window := range drained {
		kind := string(window.Window())
		// The used value the flush mirrored, not the store's current total:
		// a mismatch means a request has billed the window since, and its
		// units are not yet durable anywhere.
		byEndpoint[window.EndpointID()] = append(byEndpoint[window.EndpointID()],
			kind, quotaResetFieldPrefix+kind, window.Used())
	}
	for endpointID, args := range byEndpoint {
		scriptArgs := append([]interface{}{quotaFlushedFieldPrefix, len(args) / 3}, args...)
		if err := settleScript.Run(callCtx, s.client,
			[]string{quotaKey(endpointID)}, scriptArgs...).Err(); err != nil {
			return err
		}
	}
	return nil
}

// quotaKey names the hash holding one endpoint's counters.
func quotaKey(endpointID string) string { return quotaKeyPrefix + endpointID }

// parseReset reads a stored reset instant, returning nil when the value is
// absent or malformed. The script stores unix microseconds; an instant written
// by an older deployment as RFC 3339 text is still read, so a window written
// before the upgrade is not silently dropped. An unreadable instant leaves the
// window without one rather than failing the drain: the counter is still worth
// flushing.
func parseReset(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	if micros, err := strconv.ParseInt(raw, 10, 64); err == nil {
		utc := time.UnixMicro(micros).UTC()
		return &utc
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}
