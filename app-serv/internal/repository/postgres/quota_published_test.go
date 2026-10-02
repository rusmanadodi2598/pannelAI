// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published_test.go
// @for       The driver-error mapping, the NULL decoders, and the no-query guards
//
//	of the published-quota repository, without a server.
//
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn,
//
//	internal/domain, context, errors, testing, time.
//
// @reason    The same reasoning gateway_key_error_test.go gives for existing
//
//	without a database: a client sees a code, and a code that stops mapping is
//
// invisible until someone gets 500 where they should get 404. The two
//
//	guard clauses are here for the same reason — they are the no-N+1 and
//	no-unbounded-sweep promises, and they must hold on the default
//	`go test ./...` run, not only where a Postgres exists.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-02
package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestTranslatePublishedQuotaError pins the driver-to-domain mapping, including
// the wrap that must survive: a caller matching on the sentinel and a log that
// still shows the driver's reason are both needed.
func TestTranslatePublishedQuotaError(t *testing.T) {
	cause := errors.New("connection reset by peer")

	cases := []struct {
		name       string
		err        error
		wantIs     error
		wantText   string
		keepsCause bool
	}{
		{name: "no error passes through", err: nil},
		{name: "no rows is a missing window", err: pgx.ErrNoRows, wantIs: domain.ErrQuotaWindowNotFound},
		{
			name:   "a vanished endpoint is not-found, not a retry",
			err:    &pgconn.PgError{Code: foreignKeyViolation, ConstraintName: "quota_published_state_endpoint_id_fkey"},
			wantIs: domain.ErrEndpointNotFound,
		},
		{
			name:     "a check violation is a malformed answer",
			err:      &pgconn.PgError{Code: checkViolation, ConstraintName: "quota_published_state_failures_check"},
			wantText: "published quota window is malformed",
		},
		{
			name:     "a not-null violation is a malformed answer",
			err:      &pgconn.PgError{Code: notNullViolation, ConstraintName: "quota_published_window_fetched_at_not_null"},
			wantText: "published quota window is malformed",
		},
		{
			name:       "anything else keeps the cause and the table it came from",
			err:        cause,
			wantText:   "quota_published_state/quota_published_window",
			keepsCause: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translatePublishedQuotaError(tc.err)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("translatePublishedQuotaError(nil) = %v, want nil", got)
				}
				return
			}
			if tc.wantIs != nil && !errors.Is(got, tc.wantIs) {
				t.Fatalf("translatePublishedQuotaError() = %v, want it to match %v", got, tc.wantIs)
			}
			if tc.wantText != "" && !strings.Contains(got.Error(), tc.wantText) {
				t.Fatalf("translatePublishedQuotaError() = %q, want it to mention %q", got, tc.wantText)
			}
			// Only the unrecognised path wraps: a mapped error becomes the
			// sentinel the caller acts on, and §1.3 is precisely about keeping the
			// driver's text out of that shape.
			if tc.keepsCause && !errors.Is(got, tc.err) {
				t.Fatalf("translatePublishedQuotaError() dropped the cause: %v", got)
			}
		})
	}
}

// TestPublishedNullDecoders covers the pointer-to-value boundary the LEFT JOIN
// needs: a bucket column that is NOT NULL in the table is still NULL in a joined
// row with no bucket, and each decoder has to say what that means.
func TestPublishedNullDecoders(t *testing.T) {
	text := "pro"
	truth := true
	instant := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

	if got := derefText(nil); got != "" {
		t.Fatalf("derefText(nil) = %q, want the empty string", got)
	}
	if got := derefText(&text); got != "pro" {
		t.Fatalf("derefText(&%q) = %q, want the value", text, got)
	}
	if derefBool(nil) {
		t.Fatal("derefBool(nil) = true, want false: no bucket carries no flags")
	}
	if !derefBool(&truth) {
		t.Fatal("derefBool(&true) = false, want the stored value")
	}
	if !derefTime(nil).IsZero() {
		t.Fatal("derefTime(nil) invented an instant, want the zero time")
	}
	if got := derefTime(&instant); !got.Equal(instant) {
		t.Fatalf("derefTime() = %v, want %v", got, instant)
	}
	if optionalText("") != nil {
		t.Fatal(`optionalText("") stored an empty string, want SQL NULL`)
	}
	if got := optionalText("tokens"); got == nil || *got != "tokens" {
		t.Fatalf("optionalText(%q) = %v, want the value", "tokens", got)
	}
}

// TestPublishedQuotaRepository_GuardsRunWithoutAQuery proves the two guard clauses
// return before any statement: the repository is built over a nil pool, so a
// missing guard does not fail a query — it panics reaching one. That is the
// strongest thing a test without a server can say about "no query for an empty
// page" and "no sweep for a zero limit" (AGENTS.md §1.7).
func TestPublishedQuotaRepository_GuardsRunWithoutAQuery(t *testing.T) {
	repo := &PublishedQuotaRepository{}
	ctx := context.Background()

	answers, err := repo.ListPublishedByEndpointIDs(ctx, nil)
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs(nil) error = %v", err)
	}
	if len(answers) != 0 {
		t.Fatalf("ListPublishedByEndpointIDs(nil) returned %d endpoints, want 0", len(answers))
	}

	empty, err := repo.ListPublishedByEndpointIDs(ctx, []string{})
	if err != nil {
		t.Fatalf("ListPublishedByEndpointIDs(empty) error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("ListPublishedByEndpointIDs(empty) returned %d endpoints, want 0", len(empty))
	}

	due, err := repo.DueForRefresh(ctx, time.Now(), 0)
	if err != nil {
		t.Fatalf("DueForRefresh(limit 0) error = %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("DueForRefresh(limit 0) returned %d endpoints, want 0", len(due))
	}
}

// TestPublishedQuotaRepository_ValidationRunsWithoutAQuery pins that a malformed
// answer is refused in Go before it costs a round trip, which is what keeps the
// migration's CHECK constraints a backstop rather than the first line.
func TestPublishedQuotaRepository_ValidationRunsWithoutAQuery(t *testing.T) {
	repo := &PublishedQuotaRepository{}
	ctx := context.Background()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

	if err := repo.StorePublished(ctx, domain.PublishedAnswer{EndpointID: "ep_a", ProviderID: "alpha", FetchedAt: now,
		Windows: []domain.PublishedWindowRow{{EndpointID: "ep_a", Label: "Weekly", Used: "1"}, {EndpointID: "ep_a", Label: "Weekly", Used: "2"}}}); err == nil {
		t.Fatal("StorePublished accepted a duplicated label, which the upsert cannot write")
	}
	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: "ep_a", ProviderID: "alpha",
		AttemptedAt: now, NextAttemptAt: now.Add(time.Minute), FailureDelta: -1,
	}); err == nil {
		t.Fatal("RecordAttempt accepted a negative failure delta")
	}
	if err := repo.RecordAttempt(ctx, domain.PublishedAttempt{
		EndpointID: "", ProviderID: "alpha", AttemptedAt: now, NextAttemptAt: now, FailureDelta: 1,
	}); err == nil {
		t.Fatal("RecordAttempt accepted a blank endpoint id")
	}
}
