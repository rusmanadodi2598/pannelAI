// Package schema defines the typed request and response contracts of the API.
//
// @file      internal/schema/query.go
// @for       The typed shape of the list filters that arrive as query parameters.
// @uses      none beyond the standard library; validated through ValidateStruct.
// @reason    AGENTS.md §1.4 treats a query parameter as external input needing a typed contract,
//
//	and the handlers that read `?provider_id=`, `?capability=`, `?q=` and `?type=` were passing
//	them to the service trimmed only. A misspelled value then reads as a narrowed list while
//	narrowing nothing, which is the failure draft 025 F5 already fixed for `?active=`.
//
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

// ProviderNodeListQuery is the narrowed view of GET /api/v1/provider-nodes.
type ProviderNodeListQuery struct {
	Type string `json:"type,omitempty" validate:"omitempty,max=32"`
}
