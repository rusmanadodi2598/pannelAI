// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_retention.go
// @for       Batched removal of usage records outside the retention window.
// @uses      context, time.
// @reason    usage_records has no expiry of its own; a single unbounded DELETE would hold locks and roll back on a deadline, so the worker removes fixed batches and keeps the committed progress.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-22
package postgres

import (
	"context"
	"time"
)

// usageRetentionBatchSize bounds one DELETE; a test can lower it to pin the loop.
var usageRetentionBatchSize = 5000

// DeleteUsageOlderThan removes usage rows older than the cutoff in bounded
// batches and reports the committed count.
func (r *UsageRepository) DeleteUsageOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for {
		tag, err := r.pool.Exec(ctx, `
DELETE FROM usage_records
 WHERE ctid IN (
   SELECT ctid FROM usage_records WHERE ts < $1 ORDER BY ts LIMIT $2
 )`, cutoff, usageRetentionBatchSize)
		if err != nil {
			return total, translateUsageError(err)
		}
		deleted := tag.RowsAffected()
		total += deleted
		if deleted < int64(usageRetentionBatchSize) {
			return total, nil
		}
	}
}
