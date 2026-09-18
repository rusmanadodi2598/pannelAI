// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_scan.go
// @for       Row decoding for usage reads and the usage driver-error mapping.
// @uses      github.com/jackc/pgx/v5, internal/domain, fmt, time.
// @reason    Every usage read decodes into the same aggregate projection, so
//
//	the scan order lives beside the projection const that fixes it: a
//	single mismatch between a SELECT list and a Scan destination is an
//	opaque runtime error, and keeping both in one package makes it
//	reviewable in one place.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// scanTotals reads one aggregate row.
func scanTotals(s scanner) (domain.UsageTotals, error) {
	var totals domain.UsageTotals
	err := s.Scan(&totals.Requests, &totals.TokensIn, &totals.TokensOut,
		&totals.TokensCacheRead, &totals.TokensCacheWrite, &totals.CostUSD,
		&totals.LatencyMS, &totals.LatencyP50MS, &totals.LatencyP95MS, &totals.ErrorCount)
	if err != nil {
		return domain.UsageTotals{}, err
	}
	return totals, nil
}

// scanGroupedTotals reads a group key and its aggregate.
func scanGroupedTotals(s scanner) (domain.UsageGroupRow, error) {
	var (
		key    string
		totals domain.UsageTotals
	)
	err := s.Scan(&key, &totals.Requests, &totals.TokensIn, &totals.TokensOut,
		&totals.TokensCacheRead, &totals.TokensCacheWrite, &totals.CostUSD,
		&totals.LatencyMS, &totals.LatencyP50MS, &totals.LatencyP95MS, &totals.ErrorCount)
	if err != nil {
		return domain.UsageGroupRow{}, err
	}
	return domain.UsageGroupRow{Key: key, Totals: totals}, nil
}

// scanBucketTotals reads a bucket instant and its aggregate.
func scanBucketTotals(s scanner) (time.Time, domain.UsageTotals, error) {
	var (
		bucket time.Time
		totals domain.UsageTotals
	)
	err := s.Scan(&bucket, &totals.Requests, &totals.TokensIn, &totals.TokensOut,
		&totals.TokensCacheRead, &totals.TokensCacheWrite, &totals.CostUSD,
		&totals.LatencyMS, &totals.LatencyP50MS, &totals.LatencyP95MS, &totals.ErrorCount)
	if err != nil {
		return time.Time{}, domain.UsageTotals{}, err
	}
	return bucket, totals, nil
}

// usageRecordRow is the destination set the record projection decodes into. It
// exists so the two record scanners below share one field list: a column added
// to the projection has exactly one place to be added here, and the compiler
// then flags every call site.
type usageRecordRow struct {
	id, requestID, endpointID, providerID, gatewayKeyID string
	model, combo, status, errorCode, costUSD            string
	ts                                                  time.Time
	tokensIn, tokensOut, cacheRead, cacheWrite          int64
	latencyMS                                           int64
}

// dest is the scan destination list, in the projection's column order.
func (row *usageRecordRow) dest(extra ...any) []any {
	targets := []any{
		&row.id, &row.requestID, &row.ts, &row.endpointID, &row.providerID,
		&row.gatewayKeyID, &row.model, &row.combo, &row.tokensIn, &row.tokensOut,
		&row.cacheRead, &row.cacheWrite, &row.costUSD, &row.latencyMS,
		&row.status, &row.errorCode,
	}
	return append(targets, extra...)
}

// record rehydrates the scanned fields into the aggregate.
func (row *usageRecordRow) record() domain.UsageRecord {
	return domain.RehydrateUsageRecord(row.id, row.requestID, row.ts, row.endpointID,
		row.providerID, row.gatewayKeyID, row.model, row.combo, row.tokensIn,
		row.tokensOut, row.cacheRead, row.cacheWrite, row.costUSD, row.latencyMS,
		row.status, row.errorCode)
}

// scanUsageRecord reads one stored row into a rehydrated aggregate.
func scanUsageRecord(s scanner) (domain.UsageRecord, error) {
	var row usageRecordRow
	if err := s.Scan(row.dest()...); err != nil {
		return domain.UsageRecord{}, err
	}
	return row.record(), nil
}

// scanUsageRecordWithTotal reads a List row: the record plus the window count.
func scanUsageRecordWithTotal(s scanner) (domain.UsageRecord, int64, error) {
	var (
		row   usageRecordRow
		total int64
	)
	if err := s.Scan(row.dest(&total)...); err != nil {
		return domain.UsageRecord{}, 0, err
	}
	return row.record(), total, nil
}

// translateUsageError maps a driver error to a domain error a caller can act
// on. Unrecognised errors are wrapped with table context; the wrapped text
// reaches logs only, because the handler renders the domain message
// (AGENTS.md §1.3).
func translateUsageError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrUsageRecordNotFound
	}
	return fmt.Errorf("usage_records: %w", err)
}
