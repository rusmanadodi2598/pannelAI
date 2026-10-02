// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/published_quota.go
// @for       The published-quota cache boundary: the screen's one batched read
//
//	and the poll worker's sweep, store, and attempt bookkeeping.
//
// @uses      context, time, internal/domain.
// @reason    The quota screen must show the quota a provider publishes about
//
//	itself, and fetching it on read would fan out one provider call per
//	account — the N+1 shape AGENTS.md §1.7 blocks on this screen. So a
//	worker writes the answers into a cache and the read is one statement.
//	This boundary is what lets the service depend on that contract without
//	importing a driver (§1.5), and it is stated as a port so the sweep and
//	the read path cannot drift into two different shapes of the same table.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-02
package repository

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// PublishedQuotaRepository is the storage boundary for the published-quota cache
// (migrations/000013_published_quota). It has two very different callers: the
// screen, which reads one page of endpoints in a single statement, and the poll
// worker, which sweeps due endpoints and writes one endpoint's answer at a time.
//
// Every method here is bounded: the read takes the ids it was given, the sweep
// takes a limit, and the store is bounded by the answer it was given. Nothing on
// this boundary may loop over a collection issuing one statement per item
// (AGENTS.md §1.7).
type PublishedQuotaRepository interface {
	// ListPublishedByEndpointIDs returns the cached answer for each id, keyed by
	// endpoint id. It is the screen's single query: one statement over the batch,
	// never one per id. An id with no cached row is absent from the map, which is
	// how "the worker has not answered for this account yet" reaches the read
	// side; an empty or nil id list returns an empty map and issues no query at
	// all.
	ListPublishedByEndpointIDs(ctx context.Context, endpointIDs []string) (map[string]domain.PublishedQuota, error)

	// DueForRefresh returns at most limit endpoints whose next attempt is due by
	// now, oldest-due first — the sweep's queue, carrying the provider id and the
	// failure run each one needs to pick an interval and a backoff. A limit below
	// one asks for nothing, so no query runs.
	DueForRefresh(ctx context.Context, now time.Time, limit int) ([]domain.PublishedState, error)

	// RecordAttempt stamps one endpoint's scheduling row after a poll, whether or
	// not that poll produced buckets: last_attempt_at, the next interval, and the
	// failure delta added to the consecutive-failure run the backoff is computed
	// from. It creates the row when none exists, which is what makes a brand-new
	// endpoint schedulable before it has ever succeeded, so provider_id is
	// required. A zero delta records a good poll without touching the run; a
	// negative delta is refused by domain.PublishedAttempt rather than clamped.
	//
	// An attempt may also carry the sentence a provider answered with when it
	// published no buckets — a refused credential, an unimplemented family, an
	// empty account. That is the card's only honest content for such an account, so
	// it is stored on the state row rather than dropped: the window rows are not
	// touched, and a stale number keeps the older stamp it really has.
	RecordAttempt(ctx context.Context, attempt domain.PublishedAttempt) error

	// StorePublished writes one endpoint's answer atomically: the state envelope,
	// every bucket in it, and the deletion of any stored label this answer does
	// not carry. That prune is what stops a provider renaming a bucket from
	// leaving the old total on screen beside its replacement. Storing an answer
	// also clears the consecutive-failure run, so the backoff restarts after a
	// good poll; the next interval stays the worker's decision and is untouched
	// here.
	StorePublished(ctx context.Context, answer domain.PublishedAnswer) error
}
