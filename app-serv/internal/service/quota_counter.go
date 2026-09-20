// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_counter.go
// @for       Advancing the Redis quota counters one served request bills
//
//	against, so the window the panel reads is a number this gateway
//	actually counted (SPEC-API-001 §7.12, register G22).
//
// @uses      internal/domain, internal/repository, context, log/slog, time.
// @reason    §6 keeps quota counters in Redis and flushes them to PostgreSQL,
//
//	but the write side had no caller: the counter store was only ever
//	read and cleared by the flush worker, so every window answered zero
//	and the read routes described a table nothing filled. The counting
//	rule belongs here rather than at each accounting site, because the
//	chat, media, and embeddings planes must not disagree about which
//	windows a call advances.
//
//	This is the ingestion half of the quota vertical. It deliberately
//	does not enforce anything: whether an exhausted endpoint is skipped
//	is a selection decision, and it lives in the router.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// QuotaRecorder advances the Redis counters for one served request. It is an
// interface at the seam rather than a concrete store so a deployment without
// Redis keeps serving, and so the accounting sites can be tested without one.
type QuotaRecorder interface {
	// Add increments one endpoint's counter for one window kind, recording the
	// instant that window rolls over at.
	Add(ctx context.Context, endpointID string, kind domain.QuotaWindowKind, units int64, resetsAt time.Time) error
}

// QuotaCounter advances every window one call bills against.
//
// A nil store makes Record a no-op, which is the documented behaviour for a
// deployment that wired no counters: the data plane keeps serving and the quota
// screen stays empty rather than the process refusing to boot.
type QuotaCounter struct {
	counters repository.QuotaCounterStore
	logger   *slog.Logger
	clock    func() time.Time
}

// NewQuotaCounter binds the counter to its store. The store is optional.
func NewQuotaCounter(counters repository.QuotaCounterStore, logger *slog.Logger) *QuotaCounter {
	if logger == nil {
		logger = slog.Default()
	}
	return &QuotaCounter{counters: counters, logger: logger, clock: time.Now}
}

// Record advances one endpoint's counters by units, in every window the gateway
// accounts for.
//
// A call with no endpoint, no positive units, or no store is a no-op: an
// endpoint id is what the counter is keyed by, and a call that spent nothing
// (a refusal, a zero-token answer) has nothing to add. Writing a zero would
// create a key the flush worker then has to drain for no information.
//
// A failed increment is logged and not returned. The client already has its
// answer, and failing a served call over accounting would turn it into an error
// the client cannot act on — the same rule the usage and log writes follow.
// The counter that failed to increment is the window under-reported, which the
// panel's totals surface to an operator.
func (c *QuotaCounter) Record(ctx context.Context, endpointID string, units int64) {
	if c == nil || c.counters == nil || endpointID == "" || units <= 0 {
		return
	}
	now := c.clock()
	for _, kind := range domain.AccountingKinds() {
		// reason: the accounting write must not fail a request the client has
		// already received an answer to; the under-count is visible in the
		// window the panel reads.
		if err := c.counters.Add(ctx, endpointID, kind, units, kind.NextReset(now)); err != nil {
			c.logger.Error("menambah counter kuota gagal",
				"endpoint_id", endpointID, "window", string(kind), "units", units, "error", err)
		}
	}
}

// SetClock replaces the counter's clock so a test can pin the reset instant
// without sleeping. It is not part of the production path.
func (c *QuotaCounter) SetClock(clock func() time.Time) {
	if c != nil && clock != nil {
		c.clock = clock
	}
}
