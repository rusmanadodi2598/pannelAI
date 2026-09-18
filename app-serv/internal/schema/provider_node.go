// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/provider_node.go
// @for       Custom provider node wire contracts (SPEC-API-001 §7.4).
// @uses      go-playground/validator/v10 (the documented struct-tag exception in
//
//	AGENTS.md "Stack").
//
// @reason    AGENTS.md §2.4 CDD requires the typed contract with validation tags
//
//	before handler logic, and §7.4 fixes the node body: the prefix is a
//	model-string namespace, base_url must be an absolute http(s) URL, and
//	api_type applies to an OpenAI-compatible node only. The tags here
//	bound each field's shape; the rules that span a value's meaning live in
//	the domain constructor, so there is one spelling of each rule rather
//	than two that can drift.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

// CreateProviderNodeRequest is the body of POST /api/v1/provider-nodes (§7.4).
//
// The charset rule for prefix, the absolute-http(s) rule for base_url, and the
// api_type-per-type rule are enforced by the domain constructor: each spans fields
// or a value's meaning rather than a per-field shape, and restating them as a weaker
// tag here would let one spelling drift from the other. The handler surfaces the
// constructor's refusal as the §8 VALIDATION_ERROR.
type CreateProviderNodeRequest struct {
	Name    string `json:"name" validate:"required,min=1,max=120"`
	Prefix  string `json:"prefix" validate:"required,min=1,max=64"`
	Type    string `json:"type" validate:"required,oneof=openai-compatible anthropic-compatible"`
	APIType string `json:"api_type,omitempty" validate:"omitempty,oneof=chat responses"`
	BaseURL string `json:"base_url" validate:"required,url"`
}

// UpdateProviderNodeRequest is the body of PATCH /api/v1/provider-nodes/{id}
// (§7.4). Type and api_type are absent: they decide which wire format the node
// speaks and are part of its identity, so changing one is a new node.
type UpdateProviderNodeRequest struct {
	Name    *string `json:"name,omitempty" validate:"omitempty,min=1,max=120"`
	Prefix  *string `json:"prefix,omitempty" validate:"omitempty,min=1,max=64"`
	BaseURL *string `json:"base_url,omitempty" validate:"omitempty,url"`
}

// ProviderNodeResponse is the list and detail shape. Format reports the wire format
// the gateway will speak to this node, so the panel can show which path it routes
// through without re-deriving the rule from type and api_type.
type ProviderNodeResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	APIType   string `json:"api_type,omitempty"`
	BaseURL   string `json:"base_url"`
	Format    string `json:"format"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ProviderNodeList wraps the node list. It carries no meta block: §7.4 lists every
// node, because nodes are a hand-configured set rather than a table that grows with
// traffic.
type ProviderNodeList struct {
	Data []ProviderNodeResponse `json:"data"`
}
