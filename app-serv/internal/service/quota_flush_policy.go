// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush_policy.go
// @for       The quota flush worker's stated retry and dead-letter policy, and
//
//	the knobs that implement it.
//
// @uses      time.
// @reason    AGENTS.md §1.6 requires every worker to state its retry and
//
//	dead-letter behaviour explicitly rather than leave them implied by
//	the loop. The statement is kept beside the policy type it
//	describes, carved out of the worker file so neither concern has to
//	scroll past the other (draft 005 F3).
//
// RETRY POLICY
//
//	A failed flush is retried on the worker's next tick with the same batch.
//	There is no in-worker retry loop: the tick is the backoff, so a database
//	outage costs one failed tick rather than a hot retry storm against a
//	database already struggling. MaxAttempts bounds how many consecutive ticks
//	one batch is retried for.
//
// DEAD-LETTER POLICY
//
//	After MaxAttempts consecutive failures on the same batch, the batch is
//	dead-lettered: it is logged at error level with the batch identity and
//	the last error, and the counters are LEFT IN REDIS, so a restart or a
//	later successful tick still flushes them. Nothing is discarded, and no
//	separate dead-letter store is invented: the counters are the running
//	total in Redis, and Settle runs only after a successful write.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-20
package service

import "time"

// QuotaFlushPolicy states the flush worker's retry and dead-letter behaviour,
// which AGENTS.md §1.6 requires every worker to declare explicitly.
//
// Retry: a flush that fails is retried on the worker's next tick with the SAME
// batch, because a counter is only settled after a successful durable write.
// There is no in-worker retry loop and no exponential backoff: the tick itself
// is the backoff, so a transient database outage costs one failed tick rather
// than a hot retry storm against a database that is already struggling.
//
// Attempts are bounded per batch. After MaxAttempts consecutive failures on the
// same batch, the batch is dead-lettered: it is logged at error level with the
// batch identity and the last error, and the Redis counters are left untouched,
// so a restart or a later successful tick still flushes them. Nothing is
// silently dropped: a counter lost is a quota an operator would under-read.
type QuotaFlushPolicy struct {
	// Interval is the tick between flushes.
	Interval time.Duration
	// BatchSize bounds one flush, so a backlog is drained over several ticks
	// instead of one unbounded statement (AGENTS.md §1.7).
	BatchSize int
	// MaxAttempts is how many consecutive failures one batch is retried for
	// before it is dead-lettered and left in Redis.
	MaxAttempts int
	// Timeout bounds one flush, so a hung database cannot stall the worker loop.
	Timeout time.Duration
}

// DefaultQuotaFlushPolicy is the policy used when a caller does not supply one.
func DefaultQuotaFlushPolicy() QuotaFlushPolicy {
	return QuotaFlushPolicy{Interval: 30 * time.Second, BatchSize: 500, MaxAttempts: 5, Timeout: 10 * time.Second}
}
