//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/log_retention_integration_test.go
// @for       The retention purge half of the request-log integration suite.
// @uses      internal/domain, testing, time.
// @reason    The purge boundary is the case worth a real server: a row exactly
//
//	at the cutoff must survive, and a stub would accept whatever rule
//	the code happened to implement. Separated from the list/detail
//	tests at the AGENTS.md §1.1 line limit after the shared DSN guard
//	was introduced (the file grew past the budget then).
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-24
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestLogRepository_RetentionPurgeBoundary covers the purge at its boundary: a
// row strictly older than the cutoff is deleted, and a row exactly at the
// cutoff survives.
func TestLogRepository_RetentionPurgeBoundary(t *testing.T) {
	repo := newLogRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	cutoff := domain.RetentionCutoff(now, 7)

	cases := []struct {
		name     string
		ts       time.Time
		wantGone bool
	}{
		{"well inside the window", now.Add(-time.Hour), false},
		{"exactly at the cutoff survives", cutoff, false},
		{"one second older than the cutoff is deleted", cutoff.Add(-time.Second), true},
		{"far older than the cutoff is deleted", now.AddDate(0, 0, -30), true},
		{"in the future is not deleted", now.Add(time.Hour), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requestID := "req_" + tc.name
			seedLog(t, repo, requestID, tc.ts, domain.RequestLogSuccess, "body", "body", "")

			deleted, err := repo.DeleteOlderThan(ctx, cutoff)
			if err != nil {
				t.Fatalf("DeleteOlderThan error = %v", err)
			}
			if tc.wantGone && deleted != 1 {
				t.Fatalf("deleted = %d, want 1", deleted)
			}
			if !tc.wantGone && deleted != 0 {
				t.Fatalf("deleted = %d, want 0: a row at or after the cutoff must survive", deleted)
			}

			_, err = repo.GetByRequestID(ctx, requestID)
			gone := errorsIs(err, domain.ErrRequestLogNotFound)
			if gone != tc.wantGone {
				t.Fatalf("row gone = %v, want %v (err = %v)", gone, tc.wantGone, err)
			}
		})
	}
}

// TestLogRepository_RetentionPurgeOverAnEmptyTable covers the other end of the
// boundary: a purge with nothing to delete removes nothing and reports nothing.
// It is its own test rather than a subtest of the boundary table because that
// table deliberately leaves survivors behind, and a purge over a table that
// still holds them is a different case — the one the table above already covers.
func TestLogRepository_RetentionPurgeOverAnEmptyTable(t *testing.T) {
	repo := newLogRepo(t)
	now := time.Now().UTC().Truncate(time.Second)

	deleted, err := repo.DeleteOlderThan(context.Background(), now)
	if err != nil {
		t.Fatalf("DeleteOlderThan error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
}
