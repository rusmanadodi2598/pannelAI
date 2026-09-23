// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_row.go
// @for       The upstream_endpoints projection, its JSONB codec, and the
//
//	row-to-aggregate mapping.
//
// @uses      encoding/json, time, internal/domain.
// @reason    The table carries three jsonb columns (oauth, account,
//
//	test_status) whose payloads the aggregate exposes as typed value
//	objects. Encoding and decoding them belongs beside the column list
//	rather than inside each statement, so every read and write agrees on
//	one shape and a field added to the aggregate cannot be persisted by
//	one path and dropped by another. It lives in its own file because
//	AGENTS.md §1.1 caps a source file at 250 lines, and the JSONB codec
//	plus the statement surface do not fit in one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// endpointColumns is the projection every endpoint read uses, in scan order.
//
// The connection-parity columns (draft 017 §4.1b) are part of the projection
// rather than a second read: they are read on every endpoint load, and a second
// query for five scalar columns would be the N+1 shape AGENTS.md §1.7 forbids.
const endpointColumns = `id, provider_id, label, auth_type, priority, status,
	oauth, account, test_status, rate_limited_until, last_used_at, created_at, updated_at,
	global_priority, default_model, consecutive_use_count,
	last_error, last_error_at, error_code, proxy_pool_id`

// endpointRow is one upstream_endpoints row, kept as raw column values rather
// than as an aggregate so the page's keys can be attached from one batched read
// instead of a read per row (AGENTS.md §1.7).
type endpointRow struct {
	id, providerID, label string
	authType              string
	priority              int
	status                string
	oauthJSON             *string
	accountJSON           string
	testJSON              *string
	rateLimitedUntil      *time.Time
	lastUsedAt            *time.Time
	createdAt, updatedAt  time.Time

	// The connection-parity columns (draft 017 §4.1b).
	globalPriority      *int
	defaultModel        string
	consecutiveUseCount int
	lastError           *string
	lastErrorAt         *time.Time
	errorCode           *string
	proxyPoolID         *string
}

// scanEndpointRow reads one endpoint row, including its window count when the
// caller selected one.
func scanEndpointRow(s scanner, extra ...any) (endpointRow, error) {
	var row endpointRow
	dest := []any{
		&row.id, &row.providerID, &row.label, &row.authType, &row.priority,
		&row.status, &row.oauthJSON, &row.accountJSON, &row.testJSON,
		&row.rateLimitedUntil, &row.lastUsedAt, &row.createdAt, &row.updatedAt,
		&row.globalPriority, &row.defaultModel, &row.consecutiveUseCount,
		&row.lastError, &row.lastErrorAt, &row.errorCode, &row.proxyPoolID,
	}
	dest = append(dest, extra...)
	if err := s.Scan(dest...); err != nil {
		return endpointRow{}, err
	}
	return row, nil
}

// aggregate rebuilds the endpoint root with the keys that belong to it. A
// malformed stored JSON blob is an error rather than a silently empty value: an
// unreadable OAuth token set must be visible, not reported as "no token".
func (row endpointRow) aggregate(keys []domain.UpstreamKey) (domain.UpstreamEndpoint, error) {
	oauth, err := unmarshalOAuth(row.oauthJSON)
	if err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	account, err := unmarshalAccount(row.accountJSON)
	if err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	test, err := unmarshalTestStatus(row.testJSON)
	if err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	return domain.RehydrateUpstreamEndpoint(
		row.id, row.providerID, row.label,
		domain.UpstreamAuthType(row.authType), row.priority,
		domain.UpstreamEndpointStatus(row.status), oauth, account, test,
		row.rateLimitedUntil, row.lastUsedAt, row.createdAt, row.updatedAt, keys,
		row.parity(),
	), nil
}

// parity projects the nullable parity columns onto the aggregate's carrier.
//
// Each column is nullable so a row created before migration 000012 reads back as
// the zero value rather than failing: an operator who upgraded mid-flight must
// see their endpoints, not a scan error.
func (row endpointRow) parity() domain.EndpointParity {
	parity := domain.EndpointParity{
		DefaultModel:        row.defaultModel,
		ConsecutiveUseCount: row.consecutiveUseCount,
		LastErrorAt:         row.lastErrorAt,
	}
	if row.globalPriority != nil {
		parity.GlobalPriority = *row.globalPriority
	}
	if row.errorCode != nil {
		parity.LastErrorCode = *row.errorCode
	}
	if row.lastError != nil {
		parity.LastErrorMessage = *row.lastError
	}
	if row.proxyPoolID != nil {
		parity.ProxyPoolID = *row.proxyPoolID
	}
	return parity
}
