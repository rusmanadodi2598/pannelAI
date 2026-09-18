// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_response.go
// @for       Mapping an endpoint aggregate and its keys onto the §7.5 wire shapes.
// @uses      internal/domain, internal/schema.
// @reason    SPEC-API-001 §7.5 fixes what a client sees for an endpoint and a key,
//
//	and §6 requires the stored OAuth state to be redacted on read. The
//	mapping lives here rather than in the schema package because the
//	schema layer must stay free of domain types (AGENTS.md §1.5), and one
//	mapper keeps the list, detail, and create responses from drifting.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// toKeyResponse renders one key. Only key_hint and the health fields cross the
// wire; the stored ciphertext has no accessor and is never read here.
func toKeyResponse(key domain.UpstreamKey, now time.Time) schema.EndpointKeyResponse {
	resp := schema.EndpointKeyResponse{
		ID:                key.ID(),
		EndpointID:        key.EndpointID(),
		Label:             key.Label(),
		KeyHint:           key.Hint(),
		Priority:          key.Priority(),
		Status:            string(key.Status()),
		Available:         key.Available(now),
		LastError:         key.LastError(),
		ConsecutiveErrors: key.ConsecutiveErrors(),
		CreatedAt:         schema.Timestamp(key.CreatedAt()),
		UpdatedAt:         schema.Timestamp(key.UpdatedAt()),
	}
	if used := key.LastUsedAt(); used != nil {
		resp.LastUsedAt = ptr(schema.Timestamp(*used))
	}
	if until := key.RateLimitedUntil(); until != nil {
		resp.RateLimitedUntil = ptr(schema.Timestamp(*until))
	}
	return resp
}

// toKeyResponses renders a key slice, always non-nil so the JSON is an empty array
// rather than null.
func toKeyResponses(keys []domain.UpstreamKey, now time.Time) []schema.EndpointKeyResponse {
	out := make([]schema.EndpointKeyResponse, 0, len(keys))
	for _, key := range keys {
		out = append(out, toKeyResponse(key, now))
	}
	return out
}

// toEndpointResponse renders one endpoint. withKeys controls whether the child
// collection is included: the list response omits it so a page stays small, and
// the detail response carries it because that is where the panel renders the keys
// table.
func toEndpointResponse(endpoint domain.UpstreamEndpoint, now time.Time, withKeys bool) schema.EndpointResponse {
	resp := schema.EndpointResponse{
		ID:             endpoint.ID(),
		ProviderID:     endpoint.ProviderID(),
		Label:          endpoint.Label(),
		AuthType:       string(endpoint.AuthType()),
		Priority:       endpoint.Priority(),
		Status:         string(endpoint.Status()),
		Account:        toAccountResponse(endpoint.Account()),
		KeyCount:       len(endpoint.Keys()),
		ActiveKeyCount: endpoint.ActiveKeyCount(),
		Available:      endpoint.Available(now),
		CreatedAt:      schema.Timestamp(endpoint.CreatedAt()),
		UpdatedAt:      schema.Timestamp(endpoint.UpdatedAt()),
	}
	if oauth := endpoint.OAuth(); oauth != nil {
		resp.OAuth = toOAuthResponse(oauth)
	}
	if test := endpoint.TestStatus(); test.State != "" {
		resp.TestStatus = toTestStatusResponse(test)
	}
	if until := endpoint.RateLimitedUntil(); until != nil {
		resp.RateLimitedUntil = ptr(schema.Timestamp(*until))
	}
	if used := endpoint.LastUsedAt(); used != nil {
		resp.LastUsedAt = ptr(schema.Timestamp(*used))
	}
	if withKeys {
		resp.Keys = toKeyResponses(endpoint.Keys(), now)
	}
	return resp
}

// toAccountResponse renders the non-secret account identity.
func toAccountResponse(account domain.EndpointAccount) schema.EndpointAccountResponse {
	return schema.EndpointAccountResponse{
		Name:        account.Name,
		Email:       account.Email,
		MachineID:   account.MachineID,
		WorkspaceID: account.WorkspaceID,
	}
}

// toOAuthResponse redacts the credential set (§6: oauth is redacted on read).
//
// Presence flags stand in for the tokens, which never cross the wire: a panel that
// needs to show "a refresh token is stored" can, and nothing here can be replayed
// against the provider.
func toOAuthResponse(credential *domain.OAuthCredential) *schema.EndpointOAuthResponse {
	resp := &schema.EndpointOAuthResponse{
		Scopes:          credential.Scopes,
		ProjectID:       credential.ProjectID,
		AccountID:       credential.AccountID,
		AccountEmail:    credential.AccountEmail,
		HasAccessToken:  credential.AccessTokenEncrypted != "",
		HasRefreshToken: credential.RefreshTokenEncrypted != "",
	}
	if credential.ExpiresAt != nil {
		resp.ExpiresAt = ptr(schema.Timestamp(*credential.ExpiresAt))
	}
	if credential.LastRefreshAt != nil {
		resp.LastRefreshAt = ptr(schema.Timestamp(*credential.LastRefreshAt))
	}
	return resp
}

// toTestStatusResponse renders the last connectivity result.
func toTestStatusResponse(status domain.EndpointTestStatus) *schema.EndpointTestStatusResponse {
	resp := &schema.EndpointTestStatusResponse{
		State:     status.State,
		LatencyMS: status.LatencyMS,
		Message:   status.Message,
	}
	if status.CheckedAt != nil {
		resp.CheckedAt = ptr(schema.Timestamp(*status.CheckedAt))
	}
	return resp
}

// toProbeResponse renders a fresh probe's outcome in the same shape a stored test
// status uses, so a client parses one type for both.
func toProbeResponse(outcome service.ProbeOutcome) schema.EndpointTestStatusResponse {
	return schema.EndpointTestStatusResponse{
		State:     outcome.State,
		LatencyMS: outcome.LatencyMS,
		Message:   outcome.Message,
	}
}
