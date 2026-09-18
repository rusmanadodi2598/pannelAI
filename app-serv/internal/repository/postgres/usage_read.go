// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_read.go
// @for       The timeseries, paged records, request detail, and monthly rollup
//
//	reads over usage_records.
//
// @uses      github.com/jackc/pgx/v5, internal/domain, internal/repository.
// @reason    SPEC-API-001 §7.12 offers a bucketed chart, a paged raw list, a
//
//	single-request detail, and a budget comparison. Each is one
//	statement with an explicit range: the paged read carries its own
//	count in a window function, and the bucket width comes from the
//	closed granularity set so no caller text reaches date_bin.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// bucketOrigin anchors every date_bin bucket so the same timestamp always lands
// in the same hour or day, however far into the range it is.
const bucketOrigin = `timestamptz '2000-01-01'`

// Timeseries returns totals bucketed by the granularity, oldest first.
//
// The bucket boundary is the database's date_bin, so an hour is the database's
// hour and a day is its day; computing it in Go would let a DST transition put
// two different boundary rules in one range. Buckets with no records are absent
// rather than zero-filled, so a gap in the chart is a gap in the data.
func (r *UsageRepository) Timeseries(ctx context.Context, filter domain.UsageFilter, granularity domain.UsageGranularity) ([]domain.RateBucket, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	if !granularity.IsValid() {
		return nil, domain.NewValidationError("invalid granularity: " + string(granularity))
	}

	query := `SELECT date_bin($9::interval, ts, ` + bucketOrigin + `) AS bucket, ` +
		usageTotalsProjection + usageFilterClause + `
	 GROUP BY bucket
	 ORDER BY bucket ASC`

	rows, err := r.pool.Query(ctx, query, append(filterArgs(filter), granularity.Interval())...)
	if err != nil {
		return nil, translateUsageError(err)
	}
	defer rows.Close()

	buckets := make([]domain.RateBucket, 0, 48)
	for rows.Next() {
		bucket, totals, err := scanBucketTotals(rows)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, domain.RateBucket{Bucket: bucket, Totals: totals})
	}
	if err := rows.Err(); err != nil {
		return nil, translateUsageError(err)
	}
	return buckets, nil
}

// List returns one page of raw records newest-first. The total comes from a
// window function in the same statement, so the count and the page describe one
// snapshot; a separate count runs only when the page is past the last row and
// therefore carried no window count (SPEC-API-001 §4).
func (r *UsageRepository) List(ctx context.Context, filter domain.UsageFilter, q repository.PageQuery) ([]domain.UsageRecord, int64, error) {
	if err := filter.Validate(); err != nil {
		return nil, 0, err
	}
	query := `SELECT ` + usageRecordColumns + `, count(*) OVER() AS total` +
		usageFilterClause + `
	 ORDER BY ts DESC, id DESC
	 LIMIT $9 OFFSET $10`

	rows, err := r.pool.Query(ctx, query, append(filterArgs(filter), q.PerPage, q.Offset())...)
	if err != nil {
		return nil, 0, translateUsageError(err)
	}
	defer rows.Close()

	records := make([]domain.UsageRecord, 0, q.PerPage)
	var total int64
	for rows.Next() {
		record, rowTotal, err := scanUsageRecordWithTotal(rows)
		if err != nil {
			return nil, 0, err
		}
		total = rowTotal
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translateUsageError(err)
	}
	if len(records) == 0 {
		count, err := r.count(ctx, filter)
		if err != nil {
			return nil, 0, err
		}
		total = count
	}
	return records, total, nil
}

// GetByRequestID loads the record for one request, newest first when a request
// id somehow has more than one row. A missing row yields
// domain.ErrUsageRecordNotFound.
func (r *UsageRepository) GetByRequestID(ctx context.Context, requestID string) (domain.UsageRecord, error) {
	const q = `SELECT ` + usageRecordColumns + `
	  FROM usage_records WHERE request_id = $1 ORDER BY ts DESC LIMIT 1`

	record, err := scanUsageRecord(r.pool.QueryRow(ctx, q, requestID))
	if err != nil {
		return domain.UsageRecord{}, translateUsageError(err)
	}
	return record, nil
}

// MonthlyUsage returns the month-to-date totals for one endpoint, which is what
// a budget cap is compared against (§7.12). The window starts at the first
// instant of the calendar month in UTC, matching the cap's unit.
//
// An empty endpointID sums every endpoint, matching the collection semantics of
// ListWindows. The predicate guards the empty case before comparing, because a
// record recorded without an endpoint stores SQL NULL and NULL never equals a
// literal.
func (r *UsageRepository) MonthlyUsage(ctx context.Context, endpointID string, now time.Time) (domain.UsageTotals, error) {
	utc := now.UTC()
	monthStart := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	const q = `SELECT ` + usageTotalsProjection + `
	  FROM usage_records
	 WHERE ($1 = '' OR endpoint_id = $1) AND ts >= $2 AND ts <= $3`

	totals, err := scanTotals(r.pool.QueryRow(ctx, q, endpointID, monthStart, utc))
	if err != nil {
		return domain.UsageTotals{}, translateUsageError(err)
	}
	return totals, nil
}

// totals reads the ungrouped aggregate for a filter.
func (r *UsageRepository) totals(ctx context.Context, filter domain.UsageFilter) (domain.UsageTotals, error) {
	return scanTotals(r.pool.QueryRow(ctx, `SELECT `+usageTotalsProjection+usageFilterClause, filterArgs(filter)...))
}

// count reads the row count for a filter, used only when a page past the last
// row returned no window count.
func (r *UsageRepository) count(ctx context.Context, filter domain.UsageFilter) (int64, error) {
	var total int64
	query := `SELECT count(*)` + usageFilterClause
	if err := r.pool.QueryRow(ctx, query, filterArgs(filter)...).Scan(&total); err != nil {
		return 0, translateUsageError(err)
	}
	return total, nil
}
