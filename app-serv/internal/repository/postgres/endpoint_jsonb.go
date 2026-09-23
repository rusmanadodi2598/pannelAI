// Package postgres holds the PostgreSQL implementations of the repository
// contracts.
//
// @file      internal/repository/postgres/endpoint_jsonb.go
// @for       The stored JSONB shapes of an endpoint row: its OAuth token set, its
//
//	account block, and its test status.
//
// @uses      internal/domain, encoding/json, time.
// @reason    AGENTS.md §1.6 makes these the columns that must never carry
// //
//
//	plaintext: the OAuth token set is ciphertext at rest and a test status
//	is a probe result. Keeping the codecs together, and apart from the
//	row's scalar columns, keeps each file's reason readable and both
//	inside the §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-23
package postgres

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

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

// parityColumns projects an endpoint's parity fields onto the write column order
// both statements use, so the INSERT and the UPDATE cannot disagree about which
// column holds which value — the divergence that would silently swap a proxy for
// a model name.
//
// `global_priority` is written as a pointer so an unset order stores NULL rather
// than 0: the two mean different things, and a 0 would sort as "tried first".
func parityColumns(endpoint domain.UpstreamEndpoint) []any {
	code, message, at := endpoint.LastError()
	var priority *int
	if value := endpoint.GlobalPriority(); value != 0 {
		priority = &value
	}
	return []any{
		priority,
		endpoint.DefaultModel(),
		endpoint.ConsecutiveUseCount(),
		nullIfEmpty(message),
		at,
		nullIfEmpty(code),
		nullIfEmpty(endpoint.ProxyPoolID()),
	}
}

// nullIfEmpty stores an empty string as NULL, so "no value" has one
// representation in the schema rather than two.
func nullIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
