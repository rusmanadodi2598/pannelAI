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
	"encoding/json"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// endpointColumns is the projection every endpoint read uses, in scan order.
const endpointColumns = `id, provider_id, label, auth_type, priority, status,
	oauth, account, test_status, rate_limited_until, last_used_at, created_at, updated_at`

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
}

// scanEndpointRow reads one endpoint row, including its window count when the
// caller selected one.
func scanEndpointRow(s scanner, extra ...any) (endpointRow, error) {
	var row endpointRow
	dest := []any{
		&row.id, &row.providerID, &row.label, &row.authType, &row.priority,
		&row.status, &row.oauthJSON, &row.accountJSON, &row.testJSON,
		&row.rateLimitedUntil, &row.lastUsedAt, &row.createdAt, &row.updatedAt,
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
	), nil
}

// oauthPayload is the stored shape of upstream_endpoints.oauth. Both token
// fields hold ciphertext, so the column never carries token material
// (SPEC-API-001 §6).
type oauthPayload struct {
	AccessTokenEncrypted  string     `json:"access_token_encrypted,omitempty"`
	RefreshTokenEncrypted string     `json:"refresh_token_encrypted,omitempty"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty"`
	Scopes                []string   `json:"scopes,omitempty"`
	ProjectID             string     `json:"project_id,omitempty"`
	AccountID             string     `json:"account_id,omitempty"`
	AccountEmail          string     `json:"account_email,omitempty"`
	LastRefreshAt         *time.Time `json:"last_refresh_at,omitempty"`
}

// accountPayload is the stored shape of upstream_endpoints.account, the
// non-secret identity that distinguishes two accounts of one provider.
type accountPayload struct {
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	MachineID   string `json:"machine_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// testStatusPayload is the stored shape of upstream_endpoints.test_status.
type testStatusPayload struct {
	State     string     `json:"state"`
	LatencyMS int        `json:"latency_ms"`
	Message   string     `json:"message,omitempty"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
}

// marshalOAuth renders the credential set, or nil when the endpoint carries
// none, so a key endpoint stores SQL NULL rather than an empty object that
// would read back as "has a credential".
func marshalOAuth(credential *domain.OAuthCredential) (*string, error) {
	if credential == nil {
		return nil, nil
	}
	raw, err := json.Marshal(oauthPayload{
		AccessTokenEncrypted:  credential.AccessTokenEncrypted,
		RefreshTokenEncrypted: credential.RefreshTokenEncrypted,
		ExpiresAt:             credential.ExpiresAt,
		Scopes:                credential.Scopes,
		ProjectID:             credential.ProjectID,
		AccountID:             credential.AccountID,
		AccountEmail:          credential.AccountEmail,
		LastRefreshAt:         credential.LastRefreshAt,
	})
	if err != nil {
		return nil, fmt.Errorf("upstream_endpoints: encoding oauth: %w", err)
	}
	encoded := string(raw)
	return &encoded, nil
}

// unmarshalOAuth reads the credential set back from its stored column.
func unmarshalOAuth(raw *string) (*domain.OAuthCredential, error) {
	if raw == nil || *raw == "" || *raw == "null" {
		return nil, nil
	}
	var payload oauthPayload
	if err := json.Unmarshal([]byte(*raw), &payload); err != nil {
		return nil, fmt.Errorf("upstream_endpoints: decoding oauth: %w", err)
	}
	return &domain.OAuthCredential{
		AccessTokenEncrypted:  payload.AccessTokenEncrypted,
		RefreshTokenEncrypted: payload.RefreshTokenEncrypted,
		ExpiresAt:             payload.ExpiresAt,
		Scopes:                payload.Scopes,
		ProjectID:             payload.ProjectID,
		AccountID:             payload.AccountID,
		AccountEmail:          payload.AccountEmail,
		LastRefreshAt:         payload.LastRefreshAt,
	}, nil
}

// marshalAccount renders the account identity. The column is NOT NULL, so an
// empty identity is stored as an empty object rather than as NULL.
func marshalAccount(account domain.EndpointAccount) (string, error) {
	raw, err := json.Marshal(accountPayload{
		Name: account.Name, Email: account.Email,
		MachineID: account.MachineID, WorkspaceID: account.WorkspaceID,
	})
	if err != nil {
		return "", fmt.Errorf("upstream_endpoints: encoding account: %w", err)
	}
	return string(raw), nil
}

// unmarshalAccount reads the account identity back from its stored column.
func unmarshalAccount(raw string) (domain.EndpointAccount, error) {
	if raw == "" || raw == "null" {
		return domain.EndpointAccount{}, nil
	}
	var payload accountPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return domain.EndpointAccount{}, fmt.Errorf("upstream_endpoints: decoding account: %w", err)
	}
	return domain.EndpointAccount{
		Name: payload.Name, Email: payload.Email,
		MachineID: payload.MachineID, WorkspaceID: payload.WorkspaceID,
	}, nil
}

// marshalTestStatus renders the last connectivity result, or nil when none has
// run, so "never tested" is distinct from "tested and failed".
func marshalTestStatus(status domain.EndpointTestStatus) (*string, error) {
	if status.State == "" && status.CheckedAt == nil {
		return nil, nil
	}
	raw, err := json.Marshal(testStatusPayload{
		State: status.State, LatencyMS: status.LatencyMS,
		Message: status.Message, CheckedAt: status.CheckedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("upstream_endpoints: encoding test_status: %w", err)
	}
	encoded := string(raw)
	return &encoded, nil
}

// unmarshalTestStatus reads the connectivity result back from its stored column.
func unmarshalTestStatus(raw *string) (domain.EndpointTestStatus, error) {
	if raw == nil || *raw == "" || *raw == "null" {
		return domain.EndpointTestStatus{}, nil
	}
	var payload testStatusPayload
	if err := json.Unmarshal([]byte(*raw), &payload); err != nil {
		return domain.EndpointTestStatus{}, fmt.Errorf("upstream_endpoints: decoding test_status: %w", err)
	}
	return domain.EndpointTestStatus{
		State: payload.State, LatencyMS: payload.LatencyMS,
		Message: payload.Message, CheckedAt: payload.CheckedAt,
	}, nil
}
