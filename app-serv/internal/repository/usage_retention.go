// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/usage_retention.go
// @for       The storage boundary for time-based removal of usage records.
// @uses      context, time.
// @reason    usage_records is append-only and has no API purge route, so a background worker needs a narrow removal contract without pulling the whole usage read surface into it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-22
package repository

import (
	"context"
	"time"
)

// UsageRetentionRepository removes usage rows older than a cutoff.
type UsageRetentionRepository interface {
	// DeleteUsageOlderThan removes rows older than the cutoff in bounded batches
	// and reports the committed count.
	DeleteUsageOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
