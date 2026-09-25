// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush_drain.go
// @for       One flush cycle: read the changed windows from Redis, write them
//
//	to PostgreSQL, and settle what was written.
//
// @uses      context, fmt, log/slog, sort, internal/domain.
// @reason    The batch mechanics are the part of the flush worker whose order
//
//	a review checks against AGENTS.md §1.7 (bounded batch, set-based
//	write, settle only after durability), so they live in one file apart
//	from the worker lifecycle (draft 005 F3). Naming the batch here is
//	what makes retries attributable to the endpoints they belong to
//	(draft 005 F4).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// drain reads one batch, writes it, and settles only what was written.
func (f *QuotaFlusher) drain(ctx context.Context) {
	callCtx, cancel := context.WithTimeout(ctx, f.policy.Timeout)
	defer cancel()

	pending, err := f.counters.Pending(callCtx, f.policy.BatchSize)
	if err != nil {
		f.logger.Error("membaca counter kuota gagal", "error", err)
		return
	}
	if len(pending) == 0 {
		f.attempts, f.lastKey = 0, ""
		return
	}

	batchKey := batchIdentity(pending)
	if batchKey != f.lastKey {
		f.attempts, f.lastKey = 0, batchKey
	}

	if err := f.quotas.UpsertWindows(callCtx, pending); err != nil {
		f.attempts++
		if f.attempts >= f.policy.MaxAttempts {
			// Dead-letter: the counters stay in Redis, so they are retried after
			// a restart rather than lost. The batch identity names the endpoints
			// an operator has to look at.
			f.logger.Error("batch flush kuota masuk dead-letter",
				"batch", batchKey, "windows", len(pending), "attempts", f.attempts, "error", err)
			f.attempts, f.lastKey = 0, ""
			return
		}
		f.logger.Warn("flush kuota gagal, akan dicoba lagi",
			"batch", batchKey, "attempts", f.attempts, "error", err)
		return
	}

	if err := f.counters.Settle(callCtx, pending); err != nil {
		// The rows are durable; only the settle failed. The unsettled windows
		// are returned again next tick and rewritten, which is idempotent
		// because UpsertWindows replaces rather than adds.
		f.logger.Error("menandai counter kuota gagal", "batch", batchKey, "error", err)
		return
	}
	f.attempts, f.lastKey = 0, ""
}

// batchIdentity names a batch by the endpoint and window kind each of its
// windows bills against, so a retry of the same batch is recognised and a
// different batch is not charged with its failures. Keying on the window kind
// alone (draft 005 F4) made two failing batches of the same shape share one
// attempt count, so one batch could be dead-lettered for failures it never
// caused. The names are sorted because Pending reads live Redis state, and two
// reads of one unchanged keyspace can arrive in a different order without the
// batch having changed.
func batchIdentity(windows []domain.QuotaWindow) string {
	names := make([]string, 0, len(windows))
	for _, window := range windows {
		names = append(names, window.EndpointID()+"/"+string(window.Window()))
	}
	sort.Strings(names)
	return fmt.Sprintf("%d:%v", len(windows), names)
}
