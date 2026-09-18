// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/endpoint.go
// @for       The request contracts for upstream endpoints, their keys, and the
//
//	bulk onboarding routes (SPEC-API-001 §7.5, §8.1).
//
// @uses      go-playground/validator/v10 (the documented struct-tag exception in
//
//	AGENTS.md "Stack"), internal/domain for the shared vocabulary.
//
// @reason    AGENTS.md §2.4 CDD requires a typed struct with validation tags
//
//	before handler logic, and §8.1 settles these shapes before the
//	endpoint DTOs exist: one create shape serves the single route and
//	each element of the bulk route, the auth vocabulary crosses as
//	api_key|oauth|no_auth, and a credential is write-only so responses
//	carry a hint.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// The wire vocabulary for a credential's auth type is api_key|oauth|no_auth.
// The embedded registry's own document spells two of them differently (apikey,
// none), and a client that copies a provider's auth_type from GET /providers
// would otherwise be rejected for using the spelling the API just showed it, so
// both spellings are accepted and normalized here.
//
// `cookie` is deliberately not mapped: a browser-session credential has no
// supported transport in v1 (SPEC-API-001 §2.2), so it is a rejection rather
// than a silently accepted fourth value.

// registryAuthAliases maps the registry's spelling onto the wire vocabulary it
// stands for. The mapping is one-way and lives at the boundary, so only one
// vocabulary ever reaches the domain (SPEC-API-001 §8.1).
var registryAuthAliases = map[string]string{
	"apikey": "api_key",
	"none":   "no_auth",
}

// ParseAuthType maps a request's auth_type onto the domain vocabulary.
//
// It is the boundary §8.1 requires: the registry spells two values apikey and
// none, the API publishes api_key and no_auth, and a caller must not have to
// know both. Anything else — including cookie — is a VALIDATION_ERROR naming
// the accepted set, so the mistake is actionable rather than mysterious.
func ParseAuthType(raw string) (domain.UpstreamAuthType, error) {
	value := strings.TrimSpace(raw)
	if mapped, ok := registryAuthAliases[value]; ok {
		value = mapped
	}
	return domain.ParseUpstreamAuthType(value)
}

// TestEndpointRequest is the body of POST /api/v1/endpoints/{id}/test (§7.5). Both
// fields are optional: an absent key_id tests the first active key, which is what
// routing would spend.
type TestEndpointRequest struct {
	KeyID string `json:"key_id,omitempty" validate:"omitempty,max=64"`
}

// TestNodeRequest is the body of POST /api/v1/provider-nodes/{id}/test (§7.4). A
// node carries no credential of its own, so the caller may supply one; an absent
// value legitimately tests a node whose upstream needs none.
type TestNodeRequest struct {
	Credential string `json:"credential,omitempty" validate:"omitempty,max=8192"`
}

// EndpointKeyInput is one credential in a create or bulk body (§7.5). The value
// is write-only: no response shape echoes it back.
type EndpointKeyInput struct {
	Label    string `json:"label,omitempty" validate:"omitempty,max=120"`
	Value    string `json:"value" validate:"required,min=1,max=8192"`
	Priority int    `json:"priority,omitempty" validate:"omitempty,min=1,max=10000"`
}

// CreateEndpointRequest is the body of POST /api/v1/endpoints (§7.5). The same
// shape is the element type of the bulk route, so one validator serves both
// (§8.1).
type CreateEndpointRequest struct {
	ProviderID string             `json:"provider_id" validate:"required,min=1,max=64"`
	Label      string             `json:"label" validate:"required,min=1,max=120"`
	AuthType   string             `json:"auth_type" validate:"required,oneof=api_key oauth no_auth apikey none"`
	Priority   int                `json:"priority,omitempty" validate:"omitempty,min=1,max=10000"`
	Keys       []EndpointKeyInput `json:"keys,omitempty" validate:"omitempty,max=100,dive"`
}

// UpdateEndpointRequest is the body of PATCH /api/v1/endpoints/{id} (§7.5).
// Pointers keep "omitted" distinct from "set to empty".
type UpdateEndpointRequest struct {
	Label    *string `json:"label,omitempty" validate:"omitempty,min=1,max=120"`
	Priority *int    `json:"priority,omitempty" validate:"omitempty,min=1,max=10000"`
	Status   *string `json:"status,omitempty" validate:"omitempty,oneof=active disabled"`
}

// AddEndpointKeyRequest is the body of POST /api/v1/endpoints/{id}/keys (§7.5).
type AddEndpointKeyRequest struct {
	Label    string `json:"label,omitempty" validate:"omitempty,max=120"`
	Value    string `json:"value" validate:"required,min=1,max=8192"`
	Priority int    `json:"priority,omitempty" validate:"omitempty,min=1,max=10000"`
}

// UpdateEndpointKeyRequest is the body of PATCH
// /api/v1/endpoints/{id}/keys/{key_id} (§7.5). An omitted value keeps the stored
// credential, because the value is write-only on the wire.
type UpdateEndpointKeyRequest struct {
	Label    *string `json:"label,omitempty" validate:"omitempty,min=1,max=120"`
	Value    *string `json:"value,omitempty" validate:"omitempty,min=1,max=8192"`
	Priority *int    `json:"priority,omitempty" validate:"omitempty,min=1,max=10000"`
	Status   *string `json:"status,omitempty" validate:"omitempty,oneof=active disabled"`
}

// BulkCreateEndpointsRequest is the body of POST /api/v1/endpoints/bulk (§7.5):
// the shared provider and auth type, then one element per account.
type BulkCreateEndpointsRequest struct {
	ProviderID string              `json:"provider_id" validate:"required,min=1,max=64"`
	AuthType   string              `json:"auth_type" validate:"required,oneof=api_key oauth no_auth apikey none"`
	Endpoints  []BulkEndpointInput `json:"endpoints" validate:"required,min=1,max=50,dive"`
}

// BulkEndpointInput is one account inside a bulk create: the single-create body
// without the shared provider and auth fields (§8.1).
type BulkEndpointInput struct {
	Label    string             `json:"label" validate:"required,min=1,max=120"`
	Priority int                `json:"priority,omitempty" validate:"omitempty,min=1,max=10000"`
	Keys     []EndpointKeyInput `json:"keys,omitempty" validate:"omitempty,max=100,dive"`
}

// BulkAddKeysRequest is the body of POST /api/v1/endpoints/{id}/keys/bulk
// (§7.5). A duplicate label inside the batch is a validation failure.
type BulkAddKeysRequest struct {
	Keys []EndpointKeyInput `json:"keys" validate:"required,min=1,max=100,dive"`
}

// BulkOAuthImportRequest is the body of
// POST /api/v1/providers/{provider_id}/oauth/bulk (§7.5, §8.1): credentials
// obtained on a machine with no browser callback.
type BulkOAuthImportRequest struct {
	Accounts []BulkOAuthAccountInput `json:"accounts" validate:"required,min=1,max=50,dive"`
}

// BulkOAuthAccountInput is one already-obtained token set. Both tokens must be
// non-empty when present; they are sealed before storage and never returned.
type BulkOAuthAccountInput struct {
	Label        string               `json:"label,omitempty" validate:"omitempty,max=120"`
	AccessToken  string               `json:"access_token" validate:"required,min=1,max=8192"`
	RefreshToken string               `json:"refresh_token,omitempty" validate:"omitempty,min=1,max=8192"`
	ExpiresAt    *string              `json:"expires_at,omitempty"`
	Scopes       []string             `json:"scopes,omitempty" validate:"omitempty,max=50,dive,max=200"`
	Account      *EndpointAccountBody `json:"account,omitempty" validate:"omitempty"`
}

// EndpointAccountBody is the non-secret identity an account presents (§8.1).
type EndpointAccountBody struct {
	Name        string `json:"name,omitempty" validate:"omitempty,max=120"`
	Email       string `json:"email,omitempty" validate:"omitempty,email,max=254"`
	MachineID   string `json:"machine_id,omitempty" validate:"omitempty,max=120"`
	WorkspaceID string `json:"workspace_id,omitempty" validate:"omitempty,max=120"`
}

// EndpointAccountResponse is the account identity as returned.
type EndpointAccountResponse struct {
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	MachineID   string `json:"machine_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}
