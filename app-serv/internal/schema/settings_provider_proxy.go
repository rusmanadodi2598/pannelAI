// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_provider_proxy.go
// @for       The per-provider proxy binding on the wire: its patch shape and
//
//	the two conversions between the domain map and JSON.
//
// @uses      internal/domain (ProviderProxy).
// @reason    docs/PORT/009-PORT-PROVIDER-PROXY.md D1 puts one binding per
//
//	provider in the settings document; the type and its conversions
//	live here so settings_patch.go keeps one concern and stays under
//	the file limit (AGENTS.md §1.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-26
package schema

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"

// ProviderProxyPatch is one provider's binding on the wire. An omitted field
// inherits the global setting; the entry's "must set something" rule is the
// domain's, because a tag cannot state a cross-field rule inside a map value.
type ProviderProxyPatch struct {
	PoolID   *string `json:"pool_id,omitempty" validate:"omitempty,max=64"`
	Strategy *string `json:"strategy,omitempty" validate:"omitempty,oneof=fallback round_robin"`
}

// providerProxiesPtr lowers the per-provider binding map, whole. A nil map is
// "leave unchanged"; a non-nil one replaces every entry, so a deleted binding
// stays deleted.
func providerProxiesPtr(in *map[string]ProviderProxyPatch) *map[string]domain.ProviderProxy {
	if in == nil {
		return nil
	}
	lowered := make(map[string]domain.ProviderProxy, len(*in))
	for id, entry := range *in {
		converted := domain.ProviderProxy{}
		if entry.PoolID != nil {
			converted.PoolID = *entry.PoolID
		}
		if entry.Strategy != nil {
			converted.Strategy = *entry.Strategy
		}
		lowered[id] = converted
	}
	return &lowered
}

// providerProxiesOrEmpty renders the binding map as an object, never null, so
// a document with no bindings does not fail the panel's own form validation.
func providerProxiesOrEmpty(entries map[string]domain.ProviderProxy) map[string]ProviderProxyResponse {
	rendered := make(map[string]ProviderProxyResponse, len(entries))
	for id, entry := range entries {
		rendered[id] = ProviderProxyResponse{PoolID: entry.PoolID, Strategy: entry.Strategy}
	}
	return rendered
}
