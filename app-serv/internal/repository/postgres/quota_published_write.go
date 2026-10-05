// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published_write.go
// @for       The published-quota cache write path: storing one endpoint's answer
//
//	with its label prune, and the attempt stamp the poll worker schedules from.
//
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn,
//
//	internal/domain, context, time.
//
// @reason    The cache is the only thing the quota screen reads, so a write here
//
//	has to be all-or-nothing and set-based: a half-stored answer would show a
//	bucket from one poll beside a plan from another, and a statement per bucket
//	would make one poll of a hundred buckets a hundred round trips
//	(AGENTS.md §1.7, §2.2). These statements change for a different reason than
//	the reads in quota_published.go, a new scheduling rule, not a new display
//	column, so they are a file of their own, which also keeps each half inside
//	the §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-02
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// StorePublished writes one endpoint's provider answer in ONE transaction: the
// state envelope, every bucket upserted in one set-based statement, and the
// deletion of any stored label the answer does not carry. The prune cannot be
// dropped: a renamed bucket would otherwise leave a stale row beside its
// replacement forever. It deletes by "label not in this answer", so it costs one
// statement whatever the cache holds (§1.7). Storing an answer is the success
// signal, so the failure count resets here; no other caller can clear a failure
// without an answer. The next interval is the worker's, via RecordAttempt.
func (r *PublishedQuotaRepository) StorePublished(ctx context.Context, answer domain.PublishedAnswer) error {
	if err := answer.Validate(); err != nil {
		return err
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if err := upsertPublishedState(ctx, tx, answer); err != nil {
			return err
		}
		if err := upsertPublishedWindows(ctx, tx, answer); err != nil {
			return err
		}
		return prunePublishedWindows(ctx, tx, answer.EndpointID, answer.Labels())
	})
}

// RecordAttempt stamps one endpoint's schedule whether or not the poll produced
// buckets. It creates the row on a first attempt, which is what makes a brand-new
// endpoint schedulable at all: a sweep that only visited endpoints that had already
// succeeded would never poll one that had not.
//
// `fetched_at` is deliberately NOT written here. An attempt is not an answer, and a
// schedule stamp dressed up as a fetch stamp would tell the card a number is fresher
// than it is.
func (r *PublishedQuotaRepository) RecordAttempt(ctx context.Context, attempt domain.PublishedAttempt) error {
	if err := attempt.Validate(); err != nil {
		return err
	}

	// COALESCE rather than a plain assignment: a nil plan or message means this poll
	// stated nothing new, and the row keeps what the provider last actually said. A
	// blank overwrite would erase the one sentence the card can show for an account
	// whose provider refuses it.
	const q = `
INSERT INTO quota_published_state
    (endpoint_id, provider_id, last_attempt_at, next_attempt_at, consecutive_failures, plan, message)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (endpoint_id) DO UPDATE
   SET provider_id          = EXCLUDED.provider_id,
       last_attempt_at      = EXCLUDED.last_attempt_at,
       next_attempt_at      = EXCLUDED.next_attempt_at,
       consecutive_failures = quota_published_state.consecutive_failures
                            + EXCLUDED.consecutive_failures,
       plan                 = COALESCE(EXCLUDED.plan, quota_published_state.plan),
       message              = COALESCE(EXCLUDED.message, quota_published_state.message)`

	if _, err := r.pool.Exec(ctx, q, attempt.EndpointID, attempt.ProviderID,
		attempt.AttemptedAt, attempt.NextAttemptAt, attempt.FailureDelta,
		attempt.Plan, attempt.Message); err != nil {
		return translatePublishedQuotaError(err)
	}
	return nil
}

// upsertPublishedState writes the envelope of an answer. plan and message store
// NULL when the provider did not state one: an empty string would be a value the
// screen has to special-case, and the column is already nullable.
func upsertPublishedState(ctx context.Context, tx pgx.Tx, answer domain.PublishedAnswer) error {
	const q = `
INSERT INTO quota_published_state
    (endpoint_id, provider_id, plan, message, fetched_at)
VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5)
ON CONFLICT (endpoint_id) DO UPDATE
   SET provider_id          = EXCLUDED.provider_id,
       plan                 = EXCLUDED.plan,
       message              = EXCLUDED.message,
       fetched_at           = EXCLUDED.fetched_at,
       consecutive_failures = 0`

	_, err := tx.Exec(ctx, q, answer.EndpointID, answer.ProviderID, answer.Plan, answer.Message, answer.FetchedAt)
	return translatePublishedQuotaError(err)
}

// upsertPublishedWindows writes every bucket of an answer in one statement, the
// way the usage flush writes its batch: a poll of a hundred buckets is one round
// trip, not a hundred (§1.7). used and total travel as decimal text and are cast
// to numeric in SQL, so no value passes through a float on the way to the column
// (SPEC-API-001 §4).
//
// An answer with no buckets runs no statement here, the prune below is what
// clears the endpoint's rows in that case.
func upsertPublishedWindows(ctx context.Context, tx pgx.Tx, answer domain.PublishedAnswer) error {
	if len(answer.Windows) == 0 {
		return nil
	}
	const q = `
INSERT INTO quota_published_window
    (endpoint_id, label, used, total, unlimited, is_credit_balance, recurring, unit,
     resets_at, fetched_at)
SELECT $9::text, d.label, d.used, d.total, d.unlimited, d.is_credit_balance,
       d.recurring, d.unit, d.resets_at, $10::timestamptz
  FROM unnest($1::text[], $2::numeric[], $3::numeric[], $4::boolean[], $5::boolean[],
               $6::boolean[], $7::text[], $8::timestamptz[])
       AS d(label, used, total, unlimited, is_credit_balance, recurring, unit, resets_at)
ON CONFLICT (endpoint_id, label) DO UPDATE
   SET used      = EXCLUDED.used,
       total     = EXCLUDED.total,
       unlimited = EXCLUDED.unlimited,
       is_credit_balance = EXCLUDED.is_credit_balance,
       recurring = EXCLUDED.recurring,
       unit      = EXCLUDED.unit,
       resets_at = EXCLUDED.resets_at,
       fetched_at = EXCLUDED.fetched_at`

	labels := make([]string, 0, len(answer.Windows))
	used := make([]string, 0, len(answer.Windows))
	total := make([]*string, 0, len(answer.Windows))
	unlimited := make([]bool, 0, len(answer.Windows))
	credits := make([]bool, 0, len(answer.Windows))
	recurring := make([]bool, 0, len(answer.Windows))
	units := make([]*string, 0, len(answer.Windows))
	resets := make([]*time.Time, 0, len(answer.Windows))
	for _, window := range answer.Windows {
		labels = append(labels, window.Label)
		used = append(used, window.Used)
		total = append(total, window.Total)
		unlimited = append(unlimited, window.Unlimited)
		credits = append(credits, window.IsCreditBalance)
		recurring = append(recurring, window.Recurring)
		units = append(units, optionalText(window.Unit))
		resets = append(resets, window.ResetsAt)
	}

	_, err := tx.Exec(ctx, q, labels, used, total, unlimited, credits, recurring, units,
		resets, answer.EndpointID, answer.FetchedAt)
	return translatePublishedQuotaError(err)
}

// prunePublishedWindows deletes the buckets this endpoint has that the new answer
// does not carry. An empty label set means the provider published nothing this
// time, so every stored bucket for the endpoint goes: keeping one would be
// keeping a number the provider no longer reports.
func prunePublishedWindows(ctx context.Context, tx pgx.Tx, endpointID string, labels []string) error {
	if labels == nil {
		labels = []string{}
	}
	const q = `DELETE FROM quota_published_window WHERE endpoint_id = $1 AND NOT (label = ANY($2::text[]))`

	_, err := tx.Exec(ctx, q, endpointID, labels)
	return translatePublishedQuotaError(err)
}

// optionalText maps an absent value to SQL NULL, which is how "the provider did
// not say" is stored on every nullable text column of this cache.
func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// inTx runs fn in one transaction and rolls back on any error, so a partially
// stored answer cannot survive: the cache the screen reads is either this poll or
// the previous one (AGENTS.md §2.2).
func (r *PublishedQuotaRepository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return translatePublishedQuotaError(err)
	}
	if err := fn(tx); err != nil {
		// The rollback cannot change the outcome the caller acts on; the
		// transaction is discarded either way, so the original failure is what is
		// reported.
		_ = tx.Rollback(ctx) // reason: the fn error is the reported outcome; a rollback failure adds nothing.
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return translatePublishedQuotaError(err)
	}
	return nil
}
