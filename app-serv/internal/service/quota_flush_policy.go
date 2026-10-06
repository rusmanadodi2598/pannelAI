// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush_policy.go
// @for       The quota flush worker's stated retry and dead-letter policy, and the knobs that implement it.
// @uses      time.
// @reason    AGENTS.md §1.6 requires every worker to state its retry and dead-letter behaviour explicitly rather than leave them implied by the loop. The statement is kept beside the policy type it describes, carved out of the worker file so neither concern has to scroll past the other (draft 005 F3).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-09-20
package service

import "time"

// QuotaFlushPolicy states the flush worker's retry and dead-letter behaviour,
// which AGENTS.md §1.6 requires every worker to declare. Retry: a failed flush is
// retried on the next tick with the SAME batch, because a counter settles only
// after a successful durable write. There is no in-worker retry loop and no
// exponential backoff: the tick is the backoff, so a database outage costs one
// failed tick rather than a storm against a database already struggling. After
// MaxAttempts consecutive failures a batch is dead-lettered: logged at error with
// its identity, Redis counters left so a later tick or restart still flushes them.
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
