// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published_rows.go
// @for       The row decoders and driver-error mapping of the published-quota cache tables.
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn, internal/domain, errors, fmt, time.
// @reason    The cache is read through two shapes, a state row on its own for the sweep and a state joined to a bucket for the screen, and both decode the same columns, so the decoders belong beside each other rather than inside the queries that use them: one Scan list per shape is what keeps a added column from being decoded correctly in one path and wrongly in the other. Same reasoning as usage_scan.go and combo_rows.go, and it keeps either file inside AGENTS.md §1.1.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-02
package postgres

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// scanPublishedRow reads one joined row in a single Scan: pgx requires every
// column of a row to be taken in one call, so the state half and the window half
// are decoded together and split afterwards.
//
// The window half is decoded into pointers even where the column itself is NOT
// NULL, because a LEFT JOIN with no bucket yields NULL for all of it: an endpoint
// whose provider answered with no buckets is a real answer, not a decode failure.
// An empty label is how "this row carries no bucket" is reported to the caller.
func scanPublishedRow(s scanner) (domain.PublishedState, string, domain.PublishedWindowRow, error) {
	var (
		state         domain.PublishedState
		plan          *string
		message       *string
		fetchedAt     *time.Time
		attempted     *time.Time
		label         *string
		used          *string
		total         *string
		unlimited     *bool
		credit        *bool
		recurring     *bool
		unit          *string
		resetsAt      *time.Time
		windowFetched *time.Time
	)
	err := s.Scan(&state.EndpointID, &state.ProviderID, &plan, &message,
		&fetchedAt, &attempted, &state.NextAttemptAt, &state.ConsecutiveFailures,
		&label, &used, &total, &unlimited, &credit, &recurring, &unit, &resetsAt, &windowFetched)
	if err != nil {
		return state, "", domain.PublishedWindowRow{}, err
	}
	state.Plan = derefText(plan)
	state.Message = derefText(message)
	state.FetchedAt = fetchedAt
	state.LastAttemptAt = attempted
	if label == nil {
		return state, "", domain.PublishedWindowRow{}, nil
	}

	window := domain.PublishedWindowRow{
		EndpointID: state.EndpointID,
		Label:      *label,
		Used:       derefText(used),
		Total:      total,
		Unlimited:  derefBool(unlimited),

		// A balance and an unlimited bucket are different claims about money, and the
		// panel draws them differently: one prints an amount, the other prints a
		// spend with no bar. Collapsing them here is what would render a prepaid
		// wallet as an allowance with no ceiling (migrations/000013).
		IsCreditBalance: derefBool(credit),
		Recurring:       derefBool(recurring),
		Unit:            derefText(unit),
		ResetsAt:        resetsAt,
		FetchedAt:       derefTime(windowFetched),
	}
	return state, *label, window, nil
}

// scanPublishedState reads the state columns in publishedStateColumns order.
func scanPublishedState(s scanner) (domain.PublishedState, error) {
	var (
		state     domain.PublishedState
		plan      *string
		message   *string
		fetchedAt *time.Time
		attempted *time.Time
	)
	err := s.Scan(&state.EndpointID, &state.ProviderID, &plan, &message,
		&fetchedAt, &attempted, &state.NextAttemptAt, &state.ConsecutiveFailures)
	if err != nil {
		return domain.PublishedState{}, err
	}
	state.Plan = derefText(plan)
	state.Message = derefText(message)
	state.FetchedAt = fetchedAt
	state.LastAttemptAt = attempted
	return state, nil
}

// derefText flattens a nullable text column to the empty string, which is what
// "the provider did not say" means on this side of the cache.
func derefText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// derefBool flattens the join's NULL for a bucket column that is NOT NULL in the
// table but absent from a LEFT JOIN row: no bucket means no flag, which is false,
// not unknown.
func derefBool(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

// derefTime flattens a nullable instant to the zero time. A stored bucket always
// carries fetched_at, the column is NOT NULL, so a NULL here is not a state this
// cache can hold, and reporting the zero instant is honest where inventing one
// would hide the problem.
func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

// translatePublishedQuotaError maps a driver error raised by a published-quota
// statement to a domain error a caller can act on, wrapping anything else with
// the table it came from. The chain reaches logs only; the handler renders the
// domain message to the client (AGENTS.md §1.3).
func translatePublishedQuotaError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrQuotaWindowNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case foreignKeyViolation:
			// The endpoint vanished between the sweep and this write, so the
			// account the answer belongs to does not exist. Its cached rows went
			// with it through ON DELETE CASCADE, which makes a retry pointless.
			return domain.ErrEndpointNotFound
		case checkViolation, notNullViolation:
			return domain.NewValidationError("published quota window is malformed")
		}
	}
	return fmt.Errorf("quota_published_state/quota_published_window: %w", err)
}
