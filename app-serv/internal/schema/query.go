// Package schema defines the typed request and response contracts of the API.
//
// @file      internal/schema/query.go
// @for       The typed shape of list filters and media poll identifiers that arrive outside a JSON body.
// @uses      none beyond the standard library; validated through ValidateStruct.
// @reason    AGENTS.md §1.4 treats a query parameter and a path value as external input needing a typed contract, and the handlers that read `?provider_id=`, `?capability=`, `?q=`, `?type=`, `?status=`, and `?provider=` were passing them to the service trimmed only. A misspelled value then reads as a narrowed list while narrowing nothing, which is the failure draft 025 F5 already fixed for `?active=`.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-04
package schema

// CatalogListQuery is the narrowed view of GET /api/v1/models/catalog.
//
// Capability is bounded rather than a closed set: the values a model can carry come
// from the provider registry's own data, so an enumeration here would refuse
// something the catalog legitimately advertises. A name longer than a capability
// identifier is not that, and `q` is free text a client typed, so both get a
// ceiling instead of a vocabulary.
type CatalogListQuery struct {
	ProviderID string `json:"provider_id,omitempty" validate:"omitempty,max=64"`
	Capability string `json:"capability,omitempty" validate:"omitempty,max=32"`
	Query      string `json:"q,omitempty" validate:"omitempty,max=200"`
}

// ProviderNodeListQuery is the narrowed view of GET /api/v1/provider-nodes. Its
// one filter is a node type: unlike CatalogListQuery's capability, that set is
// closed, one spelling per wire format the gateway can translate, so this schema
// bounds the length and leaves the vocabulary to domain.NodeType.
type ProviderNodeListQuery struct {
	Type string `json:"type,omitempty" validate:"omitempty,max=32"`
}

// EndpointListQuery is the narrowed view of GET /api/v1/endpoints. Status is the
// closed lifecycle set, so a misspelled value is refused rather than reading as
// an empty page; provider_id is bounded like the create body's field.
type EndpointListQuery struct {
	ProviderID string `json:"provider_id,omitempty" validate:"omitempty,max=64"`
	Status     string `json:"status,omitempty" validate:"omitempty,oneof=active disabled error"`
}

// MediaVideoPollQuery is the narrowed view of GET /api/v1/videos/{task_id}. The
// provider is required because a poll carries no model to resolve it from, and
// the task id is bounded so a client cannot address an arbitrarily long upstream
// path.
type MediaVideoPollQuery struct {
	Provider string `json:"provider,omitempty" validate:"required,max=64"`
	Endpoint string `json:"endpoint,omitempty" validate:"omitempty,max=64"`
	TaskID   string `json:"task_id,omitempty" validate:"required,max=256"`
}
