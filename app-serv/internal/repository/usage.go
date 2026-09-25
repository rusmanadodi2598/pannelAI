// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/usage.go
// @for       Storage boundaries for usage records, quota windows, and quota caps.
// @uses      context, time, internal/domain.
// @reason    AGENTS.md §1.5 requires services to depend on these interfaces and
//
//	never on a driver. The read side is four aggregations over one
//	table, so they share one boundary: a caller cannot read the summary
//	from a different place than the records it summarizes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package repository

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// UsageRecordRepository is the storage boundary for usage accounting
// (SPEC-API-001 §7.12). Each read answers one aggregation question with one
// set-based statement: a per-group query would make a summary cost proportional
// to the number of groups (AGENTS.md §1.7).
type UsageRecordRepository interface {
	// Record appends one accounting row. It is idempotent on the row's own id,
	// so a retried reporter call cannot double-count a request.
	Record(ctx context.Context, record domain.UsageRecord) error

	// Summary returns the filter's totals and, when groupBy is set, one row per
	// distinct value of that dimension. An empty groupBy returns the totals
	// alone with no group rows.
	Summary(ctx context.Context, filter domain.UsageFilter, groupBy domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error)

	// Timeseries returns totals bucketed by the granularity, oldest bucket
	// first. Buckets with no records are absent rather than zero-filled, so a
	// gap in the chart is a gap in the data.
	Timeseries(ctx context.Context, filter domain.UsageFilter, granularity domain.UsageGranularity) ([]domain.RateBucket, error)

	// List returns one page of raw records, newest first, with the total count
	// the response meta block needs.
	List(ctx context.Context, filter domain.UsageFilter, q PageQuery) (records []domain.UsageRecord, total int64, err error)

	// GetByRequestID loads the record for one request. A missing row must yield
	// domain.ErrUsageRecordNotFound so callers map it to 404.
	GetByRequestID(ctx context.Context, requestID string) (domain.UsageRecord, error)

	// MonthlyUsage returns the month-to-date totals for one endpoint, which is
	// what a budget cap is compared against (§7.12).
	MonthlyUsage(ctx context.Context, endpointID string, now time.Time) (domain.UsageTotals, error)
}

// QuotaRepository is the storage boundary for quota windows and budget caps
// (SPEC-API-001 §7.12). Windows are written by the flush worker and read by the
// panel; caps are written by the panel and read by the router.
type QuotaRepository interface {
	// ListWindows returns the windows for one endpoint, ordered by kind. An
	// empty endpointID returns every endpoint's windows, which is what the
	// collection route reads.
	ListWindows(ctx context.Context, endpointID string) ([]domain.QuotaWindow, error)

	// UpsertWindows persists flushed counters. It is set-based: one statement
	// writes the whole batch, so flushing a thousand endpoints is one round
	// trip rather than a thousand (AGENTS.md §1.7).
	UpsertWindows(ctx context.Context, windows []domain.QuotaWindow) error

	// GetCap loads one endpoint's budget cap. A missing row must yield
	// domain.ErrQuotaCapNotFound.
	GetCap(ctx context.Context, endpointID string) (domain.QuotaCap, error)

	// SetCap replaces one endpoint's budget cap.
	SetCap(ctx context.Context, cap domain.QuotaCap) error
}

// QuotaCounterStore is the Redis-side running total the flush worker mirrors
// into PostgreSQL. Totals are advanced on every served request and mirrored in
// batches, because a synchronous write would put a database round trip on the
// request path (SPEC-API-001 §6: counters cached in Redis, flushed to PG).
//
// The store reports whether it is available so a caller can decide what to do
// when Redis is down rather than failing the request it was accounting for.
type QuotaCounterStore interface {
	// Add advances one endpoint's running total for a window kind and records
	// the instant that window rolls over at, restarting the window when the
	// previous one has closed.
	Add(ctx context.Context, endpointID string, kind domain.QuotaWindowKind, units int64, resetsAt time.Time) error

	// Pending returns the windows whose total has not been mirrored yet, at
	// most limit of them. It never returns an unbounded set.
	Pending(ctx context.Context, limit int) ([]domain.QuotaWindow, error)

	// Settle records what a flush has made durable: a window whose total is
	// unchanged is marked mirrored, and one that has also closed is retired.
	// An unsettled window is written again rather than lost.
	Settle(ctx context.Context, drained []domain.QuotaWindow) error
}
