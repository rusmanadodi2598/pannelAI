// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/log.go
// @for       PostgreSQL persistence for request logs, including capture-aware insert and retention purge.
// @uses      github.com/jackc/pgx/v5, internal/domain, internal/repository.
// @reason    SPEC-API-001 §7.13 lists logs newest first, omits bodies from the list so a page is not megabytes, and deletes rows older than the retention setting. The list is always ranged and paged, and the purge is one statement with a cutoff (AGENTS.md §1.7).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-18
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// logListColumns is the list projection: deliberately without the two body
// columns, so a page of rows stays small. The detail read selects them.
const logListColumns = `request_id, ts, coalesce(gateway_key_id, ''), coalesce(endpoint_id, ''),
	coalesce(provider_id, ''), coalesce(model, ''), status, latency_ms, coalesce(error, '')`

// logDetailColumns is the detail projection, including the captured bodies.
const logDetailColumns = logListColumns + `, coalesce(request_body, ''), coalesce(response_body, '')`

// logFilterClause is the shared FROM and WHERE, always ranged so no log query is
// ever unbounded. An empty parameter disables its filter, and the free-text `q`
// matches a case-insensitive substring of the request id, the error text, and
// the model, because the panel's placeholder promises a request id and an error
// code. A NULL error needs no coalesce for the same reason a NULL error_code
// does not on the usage read: `false OR NULL` excludes a row exactly as
// `false` does, so the OR can only add a match. Keeping the FROM here means a
// read cannot reference a bare column by mistake.
const logFilterClause = `
	  FROM request_logs
	 WHERE ts >= $1 AND ts <= $2
	   AND ($3 = '' OR status = $3)
	   AND ($4 = '' OR endpoint_id = $4)
	   AND ($5 = '' OR model = $5)
	   AND ($6 = '' OR gateway_key_id = $6)
	   AND ($7 = '' OR request_id ILIKE '%' || $7 || '%'
	                OR error ILIKE '%' || $7 || '%'
	                OR model ILIKE '%' || $7 || '%')`

// LogRepository persists captured request logs.
type LogRepository struct {
	pool *pgxpool.Pool
}

// NewLogRepository binds the repository to a pool whose limits are set
// explicitly at construction (AGENTS.md §1.7).
func NewLogRepository(pool *pgxpool.Pool) *LogRepository {
	return &LogRepository{pool: pool}
}

// Insert appends one log row under a server-generated id. A retried write may
// duplicate the audit row, which is safer than letting a reused request id
// suppress a second event.
func (r *LogRepository) Insert(ctx context.Context, entry domain.RequestLog) error {
	const q = `
INSERT INTO request_logs
    (id, request_id, ts, gateway_key_id, endpoint_id, provider_id, model, status,
     latency_ms, request_body, response_body, error)
VALUES ($1, $2, $3, nullif($4, ''), nullif($5, ''), nullif($6, ''), nullif($7, ''),
        $8, $9, nullif($10, ''), nullif($11, ''), nullif($12, ''))`

	if _, err := r.pool.Exec(ctx, q,
		domain.NewULID(entry.TS()), entry.RequestID(), entry.TS(), entry.GatewayKeyID(),
		entry.EndpointID(), entry.ProviderID(), entry.Model(), entry.Status(),
		entry.LatencyMS(), entry.RequestBody(), entry.ResponseBody(), entry.Error()); err != nil {
		return translateLogError(err)
	}
	return nil
}

// List returns one page of rows without their bodies, newest first. The total
// comes from a window function in the same statement so the count and the page
// describe one snapshot, with a separate count only when the page is past the
// last row and carried no window count (SPEC-API-001 §4).
func (r *LogRepository) List(ctx context.Context, filter domain.LogFilter, q repository.PageQuery) ([]domain.RequestLog, int64, error) {
	query := `SELECT ` + logListColumns + `, count(*) OVER() AS total` + logFilterClause + `
	 ORDER BY ts DESC, request_id DESC
	 LIMIT $8 OFFSET $9`

	rows, err := r.pool.Query(ctx, query, append(logFilterArgs(filter), q.PerPage, q.Offset())...)
	if err != nil {
		return nil, 0, translateLogError(err)
	}
	defer rows.Close()

	entries := make([]domain.RequestLog, 0, q.PerPage)
	var total int64
	for rows.Next() {
		entry, rowTotal, err := scanLogListRow(rows)
		if err != nil {
			return nil, 0, err
		}
		total = rowTotal
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		// A driver error mid-stream ends Next() quietly. Without this the page
		// that was truncated by the failure is answered as a complete one.
		return nil, 0, translateLogError(err)
	}
	if len(entries) == 0 {
		count, err := r.count(ctx, filter)
		if err != nil {
			return nil, 0, err
		}
		total = count
	}
	return entries, total, nil
}

// GetByRequestID loads the newest row including its captured bodies. A client
// may reuse a request id, so the id alone no longer names one row. A missing row
// yields domain.ErrRequestLogNotFound.
func (r *LogRepository) GetByRequestID(ctx context.Context, requestID string) (domain.RequestLog, error) {
	entry, err := scanLogDetailRow(r.pool.QueryRow(ctx,
		`SELECT `+logDetailColumns+` FROM request_logs WHERE request_id = $1
		 ORDER BY ts DESC LIMIT 1`, requestID))
	if err != nil {
		return domain.RequestLog{}, translateLogError(err)
	}
	return entry, nil
}

// retentionBatchSize bounds one DELETE, so a backlog larger than the worker's
// timeout still makes progress: each batch commits before the next one starts.
var retentionBatchSize = 5000

// DeleteOlderThan removes rows strictly older than the cutoff in bounded batches
// and reports the committed count, which survives a deadline between batches.
func (r *LogRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for {
		tag, err := r.pool.Exec(ctx, `
DELETE FROM request_logs
 WHERE ctid IN (
   SELECT ctid FROM request_logs WHERE ts < $1 ORDER BY ts LIMIT $2
 )`, cutoff, retentionBatchSize)
		if err != nil {
			return total, translateLogError(err)
		}
		deleted := tag.RowsAffected()
		total += deleted
		if deleted < int64(retentionBatchSize) {
			return total, nil
		}
	}
}

// TrimTo removes the oldest rows beyond keep, in the same bounded batches, so
// a burst between retention runs cannot grow the table without limit.
func (r *LogRepository) TrimTo(ctx context.Context, keep int) (int64, error) {
	if keep < 0 {
		keep = 0
	}
	var total int64
	for {
		var count int64
		if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM request_logs`).Scan(&count); err != nil {
			return total, translateLogError(err)
		}
		excess := count - int64(keep)
		if excess <= 0 {
			return total, nil
		}
		limit := excess
		if limit > int64(retentionBatchSize) {
			limit = int64(retentionBatchSize)
		}
		tag, err := r.pool.Exec(ctx, `
DELETE FROM request_logs
 WHERE ctid IN (
   SELECT ctid FROM request_logs ORDER BY ts ASC LIMIT $1
 )`, limit)
		if err != nil {
			return total, translateLogError(err)
		}
		deleted := tag.RowsAffected()
		total += deleted
		if deleted < limit {
			return total, nil
		}
	}
}

// logFilterArgs renders a filter as the seven positional arguments the
// predicate shares, so the predicate exists once rather than per caller.
func logFilterArgs(filter domain.LogFilter) []any {
	return []any{
		filter.From, filter.To, filter.Status, filter.EndpointID,
		filter.Model, filter.GatewayKey, filter.Query,
	}
}

// count reads the row count for a filter, used only when a page past the last
// row returned no window count.
func (r *LogRepository) count(ctx context.Context, filter domain.LogFilter) (int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*)`+logFilterClause,
		logFilterArgs(filter)...).Scan(&total); err != nil {
		return 0, translateLogError(err)
	}
	return total, nil
}
