//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_harness_integration_test.go
// @for       Shared harness for the tagged usage integration tests: connect,
//
//	apply migrations, and seed records.
//
// @uses      github.com/jackc/pgx/v5/pgxpool, internal/domain, internal/migrations,
//
//	context, os, testing, time.
//
// @reason    Both usage integration files need the same connection, the same
//
//	migration step, and the same seeding, so they share one harness
//	rather than each carrying a copy that can drift. A stub cannot
//	prove the schema this vertical reads exists; a real server can.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// newUsageRepo connects, applies migrations, and returns a repository with a
// clean usage table.
func newUsageRepo(t *testing.T) *UsageRepository {
	t.Helper()
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Fatalf("%s must be set when running with -tags=integration", testDSNEnv)
	}
	if err := migrations.Apply(context.Background(), dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `TRUNCATE usage_records`); err != nil {
		t.Fatalf("truncating usage_records: %v", err)
	}
	return NewUsageRepository(pool)
}

// seedUsage inserts one record with the given identity, model, counters, and
// cost, failing the test on a rejected construction.
func seedUsage(t *testing.T, repo *UsageRepository, requestID, providerID, model, cost string, tokensIn int64, status domain.UsageStatus) domain.UsageRecord {
	t.Helper()
	record, err := domain.NewUsageRecord(domain.UsageRecordInput{
		RequestID:  requestID,
		TS:         time.Now().UTC().Add(-time.Minute),
		ProviderID: providerID,
		Model:      model,
		Status:     status,
		TokensIn:   tokensIn,
		CostUSD:    cost,
		LatencyMS:  100,
	}, "", time.Now())
	if err != nil {
		t.Fatalf("building record %s: %v", requestID, err)
	}
	if err := repo.Record(context.Background(), record); err != nil {
		t.Fatalf("recording %s: %v", requestID, err)
	}
	return record
}

// usageRange returns a window covering the seeded records.
func usageRange() domain.UsageFilter {
	to := time.Now().UTC().Add(time.Hour)
	return domain.NewUsageFilter(domain.UsageFilterInput{From: ptrTime(to.Add(-24 * time.Hour)), To: &to}, time.Now())
}

// pageQuery builds the bounded pagination every usage read accepts.
func pageQuery(page, perPage int) repository.PageQuery {
	return repository.PageQuery{Page: page, PerPage: perPage}
}

// errorsIs wraps errors.Is so the integration tests read as one assertion.
func errorsIs(err, target error) bool { return errors.Is(err, target) }

// ptrTime returns a pointer to an instant for a filter literal.
func ptrTime(value time.Time) *time.Time { return &value }
