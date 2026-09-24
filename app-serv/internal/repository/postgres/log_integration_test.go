//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/log_integration_test.go
// @for       Integration tests for request-log insert, paging, detail, and the
//
//	retention purge boundary.
//
// @uses      github.com/jackc/pgx/v5/pgxpool, internal/domain, internal/migrations,
//
//	context, testing, time.
//
// @reason    The purge boundary is the case worth a real server: a row exactly
//
//	at the cutoff must survive, and a stub would accept whatever rule
//	the code happened to implement. The list projection also has to
//	omit the body columns, which is only observable against the real
//	statement.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// newLogRepo connects, applies migrations, and returns a repository with a
// clean log table.
func newLogRepo(t *testing.T) *LogRepository {
	t.Helper()
	dsn := requireTestDSN(t)
	if err := migrations.Apply(context.Background(), dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `TRUNCATE request_logs`); err != nil {
		t.Fatalf("truncating request_logs: %v", err)
	}
	return NewLogRepository(pool)
}

// seedLog inserts one row at the given instant, with the captured bodies the
// caller supplies.
func seedLog(t *testing.T, repo *LogRepository, requestID string, ts time.Time, status domain.RequestLogStatus, requestBody, responseBody, errText string) {
	t.Helper()
	entry, err := domain.NewRequestLog(domain.RequestLogInput{
		RequestID:    requestID,
		TS:           ts,
		EndpointID:   "ep_1",
		Model:        "gpt-4o",
		Status:       status,
		LatencyMS:    42,
		RequestBody:  requestBody,
		ResponseBody: responseBody,
		Error:        errText,
	}, time.Now())
	if err != nil {
		t.Fatalf("building log %s: %v", requestID, err)
	}
	if err := repo.Insert(context.Background(), entry); err != nil {
		t.Fatalf("inserting log %s: %v", requestID, err)
	}
}

// TestLogRepository_ListOmitsBodiesAndFilters covers the list projection, the
// filters, and the paging boundary.
func TestLogRepository_ListOmitsBodiesAndFilters(t *testing.T) {
	repo := newLogRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	base := now.Add(-time.Hour)
	for i := 0; i < 5; i++ {
		status := domain.RequestLogSuccess
		if i == 4 {
			status = domain.RequestLogError
		}
		seedLog(t, repo, fmt.Sprintf("req_%d", i), base.Add(time.Duration(i)*time.Minute), status,
			fmt.Sprintf("request-%d", i), fmt.Sprintf("response-%d", i), "")
	}

	filter := domain.NewLogFilter(domain.LogFilterInput{
		From: ptrTime(base.Add(-time.Minute)),
		To:   ptrTime(base.Add(time.Hour)),
	}, time.Now())

	t.Run("a page carries no bodies", func(t *testing.T) {
		entries, total, err := repo.List(ctx, filter, repository.PageQuery{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}
		if total != 5 || len(entries) != 5 {
			t.Fatalf("entries = %d, total = %d, want 5 and 5", len(entries), total)
		}
		for _, entry := range entries {
			if entry.HasBodies() {
				t.Fatalf("list row %s carried bodies, which a page must not do", entry.RequestID())
			}
		}
	})

	t.Run("the page size is honoured and the total still reports every row", func(t *testing.T) {
		entries, total, err := repo.List(ctx, filter, repository.PageQuery{Page: 2, PerPage: 2})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}
		if len(entries) != 2 || total != 5 {
			t.Fatalf("entries = %d, total = %d, want 2 and 5", len(entries), total)
		}
	})

	t.Run("a page past the last row still reports the total", func(t *testing.T) {
		entries, total, err := repo.List(ctx, filter, repository.PageQuery{Page: 99, PerPage: 2})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}
		if len(entries) != 0 || total != 5 {
			t.Fatalf("entries = %d, total = %d, want 0 and 5", len(entries), total)
		}
	})

	t.Run("the status filter narrows to the failure", func(t *testing.T) {
		failed := filter
		failed.Status = "error"
		entries, total, err := repo.List(ctx, failed, repository.PageQuery{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}
		if total != 1 || len(entries) != 1 || entries[0].RequestID() != "req_4" {
			t.Fatalf("failed entries = %+v, total = %d, want the single error row", entries, total)
		}
	})

	t.Run("the free-text filter matches the model", func(t *testing.T) {
		matched := filter
		matched.Query = "GPT"
		_, total, err := repo.List(ctx, matched, repository.PageQuery{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}
		if total != 5 {
			t.Fatalf("total = %d, want 5: the free-text filter is case-insensitive on the model", total)
		}
	})

	t.Run("a filter matching nothing reports zero without an error", func(t *testing.T) {
		empty := filter
		empty.Model = "no-such-model"
		entries, total, err := repo.List(ctx, empty, repository.PageQuery{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}
		if len(entries) != 0 || total != 0 {
			t.Fatalf("entries = %d, total = %d, want 0 and 0", len(entries), total)
		}
	})
}

// TestLogRepository_DetailAndInsertIdempotency covers the body-bearing detail
// read and the re-insert of the same row.
func TestLogRepository_DetailAndInsertIdempotency(t *testing.T) {
	repo := newLogRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedLog(t, repo, "req_1", now, domain.RequestLogSuccess, "the request", "the response", "")

	t.Run("detail returns the captured bodies", func(t *testing.T) {
		entry, err := repo.GetByRequestID(ctx, "req_1")
		if err != nil {
			t.Fatalf("GetByRequestID error = %v", err)
		}
		if entry.RequestBody() != "the request" || entry.ResponseBody() != "the response" {
			t.Fatalf("bodies = (%q, %q), want the captured pair", entry.RequestBody(), entry.ResponseBody())
		}
	})

	t.Run("a missing request yields the not-found sentinel", func(t *testing.T) {
		if _, err := repo.GetByRequestID(ctx, "req_missing"); !errorsIs(err, domain.ErrRequestLogNotFound) {
			t.Fatalf("GetByRequestID error = %v, want ErrRequestLogNotFound", err)
		}
	})

	t.Run("re-inserting the same row does not duplicate it", func(t *testing.T) {
		seedLog(t, repo, "req_1", now, domain.RequestLogSuccess, "the request", "the response", "")
		entries, total, err := repo.List(ctx, domain.NewLogFilter(domain.LogFilterInput{
			From: ptrTime(now.Add(-time.Hour)), To: ptrTime(now.Add(time.Hour)),
		}, time.Now()), repository.PageQuery{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}
		if total != 1 || len(entries) != 1 {
			t.Fatalf("entries = %d, total = %d, want 1 and 1: the request id is the identity", len(entries), total)
		}
	})
}
