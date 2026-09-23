// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/provider_validate.go
// @for       The §7.4 stateless credential-check contracts.
// @uses      standard library only.
// @reason    Draft 017 §4.6 adds two routes that validate a credential before a
//
//	row exists, so the request carries the destination and the credential
//	together and nothing is stored. The response carries the method that
//	proved the answer, because "valid" from a models probe and "valid" from
//	a chat probe are different evidence.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-23
package schema

// ValidateNodeRequest is the body of POST /api/v1/provider-nodes/validate.
//
// `type` is required and closed because it selects the status rule: an Anthropic
// wire treats anything but 401/403 as proof the key was accepted, while an
// OpenAI wire reads a non-2xx as a failure. Guessing the type would apply the
// wrong rule and report a working credential as rejected.
type ValidateNodeRequest struct {
	BaseURL    string `json:"base_url"    validate:"required,url,max=2048"`
	Type       string `json:"type"        validate:"required,oneof=openai-compatible anthropic-compatible"`
	APIType    string `json:"api_type,omitempty" validate:"omitempty,oneof=chat responses"`
	Credential string `json:"credential,omitempty" validate:"omitempty,max=8192"`
	ModelID    string `json:"model_id,omitempty" validate:"omitempty,max=200"`
}

// ValidateProviderRequest is the body of POST /api/v1/providers/validate.
type ValidateProviderRequest struct {
	ProviderID string `json:"provider_id" validate:"required,max=200"`
	Credential string `json:"credential,omitempty" validate:"omitempty,max=8192"`
	ModelID    string `json:"model_id,omitempty" validate:"omitempty,max=200"`
}

// ValidateResponse is the answer both routes give.
//
// It carries no credential material: `message` is built from a fixed English
// template plus the upstream status, never from an upstream body, because a body
// can echo the presented key back (OWASP A09).
type ValidateResponse struct {
	State     string `json:"state"`
	Method    string `json:"method,omitempty"`
	LatencyMS int    `json:"latency_ms"`
	Status    int    `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
}
