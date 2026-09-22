// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/log_scan.go
// @for       Row decoding for the request-log reads and the log driver-error mapping.
// @uses      github.com/jackc/pgx/v5, internal/domain, errors, fmt, time.
// @reason    Every log read decodes into the same column order the projections
//
//	fix, and the detail read adds the two body columns. Keeping the
//	scanners beside each other makes a mismatch between a SELECT list
//	and its Scan destinations reviewable in one place, and it mirrors
//	the usage reads' scan file so both verticals read the same way.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package postgres

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// logRow is the destination set the list projection decodes into. Keeping the
// non-body columns in one struct means the two scanners below cannot disagree
// about their order.
type logRow struct {
	requestID, gatewayKeyID, endpointID, providerID, model, status, errText string
	ts                                                                      time.Time
	latencyMS                                                               int64
}

// entry rehydrates the scanned fields, with the bodies the detail read adds.
func (row *logRow) entry(requestBody, responseBody string) domain.RequestLog {
	return domain.RehydrateRequestLog(row.requestID, row.ts, row.gatewayKeyID,
		row.endpointID, row.providerID, row.model, domain.RequestLogStatus(row.status),
		row.latencyMS, requestBody, responseBody, row.errText)
}

// scanLogListRow reads one list row plus the window count.
func scanLogListRow(s scanner) (domain.RequestLog, int64, error) {
	var (
		row   logRow
		total int64
	)
	if err := s.Scan(&row.requestID, &row.ts, &row.gatewayKeyID, &row.endpointID,
		&row.providerID, &row.model, &row.status, &row.latencyMS, &row.errText, &total); err != nil {
		return domain.RequestLog{}, 0, err
	}
	return row.entry("", ""), total, nil
}

// scanLogDetailRow reads one detail row including its captured bodies.
func scanLogDetailRow(s scanner) (domain.RequestLog, error) {
	var (
		row                       logRow
		requestBody, responseBody string
	)
	if err := s.Scan(&row.requestID, &row.ts, &row.gatewayKeyID, &row.endpointID,
		&row.providerID, &row.model, &row.status, &row.latencyMS, &row.errText,
		&requestBody, &responseBody); err != nil {
		return domain.RequestLog{}, err
	}
	return row.entry(requestBody, responseBody), nil
}

// translateLogError maps a driver error to a domain error a caller can act on,
// wrapping anything unrecognised with table context for the log (AGENTS.md
// §1.3).
func translateLogError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrRequestLogNotFound
	}
	return fmt.Errorf("request_logs: %w", err)
}
