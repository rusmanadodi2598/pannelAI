// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/oauth.go
// @for       The provider OAuth flow contracts: start, callback, status, refresh
//
//	(SPEC-API-001 §7.4).
//
// @uses      encoding/json, internal/domain.
// @reason    §7.4 pins the OAuth routes as P2 and §6 forbids token material in
//
//	any response, so the shapes are defined by what they must NOT carry:
//	start returns a URL and a state only, callback returns identity and a
//	hint, and status reports expiry without the tokens themselves.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// OAuthStartRequest is the body of POST /api/v1/providers/{id}/oauth/start.
// A redirect_uri is optional: a provider that pins its own callback ignores it.
type OAuthStartRequest struct {
	RedirectURI string `json:"redirect_uri,omitempty" validate:"omitempty,url,max=2048"`
}

// OAuthStartResponse is the answer a browser follows to authorize.
type OAuthStartResponse struct {
	AuthorizeURL string `json:"authorize_url"`
	State        string `json:"state"`
}

// OAuthConnectResult is the 200 JSON body a headless callback caller receives:
// the endpoint that now stands for the account, with a token hint only.
type OAuthConnectResult struct {
	EndpointID   string `json:"endpoint_id"`
	ProviderID   string `json:"provider_id"`
	Label        string `json:"label"`
	AccountEmail string `json:"account_email,omitempty"`
	TokenHint    string `json:"token_hint"`
	Created      bool   `json:"created"`
}

// OAuthEndpointStatus is one OAuth endpoint's token state.
type OAuthEndpointStatus struct {
	EndpointID    string  `json:"endpoint_id"`
	Label         string  `json:"label"`
	Status        string  `json:"status"`
	ExpiresAt     *string `json:"expires_at,omitempty"`
	LastRefreshAt *string `json:"last_refresh_at,omitempty"`
	// RefreshState is derived from the provider's refresh lead: "missing"
	// (no expiry known), "fresh", "due" (inside the lead window), or
	// "expired".
	RefreshState string `json:"refresh_state"`
}

// OAuthStatusResponse answers GET /api/v1/providers/{id}/oauth/status.
type OAuthStatusResponse struct {
	ProviderID string                `json:"provider_id"`
	Flow       string                `json:"flow"`
	Endpoints  []OAuthEndpointStatus `json:"endpoints"`
}

// OAuthRefreshRequest forces one endpoint's token refresh. An empty endpoint_id
// refreshes every due endpoint of the provider.
type OAuthRefreshRequest struct {
	EndpointID string `json:"endpoint_id,omitempty" validate:"omitempty,max=64"`
}

// OAuthRefreshResponse reports what a forced refresh did.
type OAuthRefreshResponse struct {
	Refreshed   int      `json:"refreshed"`
	EndpointIDs []string `json:"endpoint_ids"`
	ExpiresAt   *string  `json:"expires_at,omitempty"`
}

// DecodeOAuthStartRequest decodes a start body into its typed contract.
func DecodeOAuthStartRequest(raw []byte) (OAuthStartRequest, error) {
	var req OAuthStartRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return OAuthStartRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}

// DecodeOAuthRefreshRequest decodes a refresh body into its typed contract.
func DecodeOAuthRefreshRequest(raw []byte) (OAuthRefreshRequest, error) {
	var req OAuthRefreshRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return OAuthRefreshRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}
