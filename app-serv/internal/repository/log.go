// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/log.go
// @for       The RequestLogRepository boundary for captured request logs.
// @uses      context, time, internal/domain.
// @reason    SPEC-API-001 §7.13 stores request logs with bodies only when capture is on, and deletes rows older than the retention setting; both are storage operations the service must invoke without knowing which driver implements them (AGENTS.md §1.5).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-18
package repository

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// RequestLogRepository is the storage boundary for request logs
// (SPEC-API-001 §7.13).
type RequestLogRepository interface {
	// Insert appends one log row under a server-generated id. A client may reuse
	// a request id, so a second write is a second audit row rather than a
	// suppressed one.
	Insert(ctx context.Context, entry domain.RequestLog) error

	// List returns one page of rows without their bodies, newest first, with
	// the total count the meta block needs. Bodies are omitted because a page
	// of captured payloads is megabytes the list never displays.
	List(ctx context.Context, filter domain.LogFilter, q PageQuery) (entries []domain.RequestLog, total int64, err error)

	// GetByRequestID loads one row including its captured bodies. A missing row
	// must yield domain.ErrRequestLogNotFound.
	GetByRequestID(ctx context.Context, requestID string) (domain.RequestLog, error)

	// DeleteOlderThan removes rows older than the cutoff in bounded batches and
	// reports how many were committed, which is what both the retention job and
	// the purge route report. A deadline that fires between batches returns the
	// partial count with the error.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)

	// TrimTo removes the oldest rows beyond keep, which bounds the table between
	// retention runs.
	TrimTo(ctx context.Context, keep int) (int64, error)
}
