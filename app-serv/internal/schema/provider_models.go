// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/provider_models.go
// @for       The §7.4 model-list mappers: a registry entry's models onto the wire
//
//	shape, and a provider's stored account counts onto the status summary.
//
// @uses      internal/domain, internal/registry.
// @reason    The model list is the one §7.4 read whose body names its own origin
//
//	(§4.9), and draft 017 §4.5 removed the constant `suggested` flag from
//	it. Keeping the mapping apart from the DTO declarations keeps the
//	contract readable in one file and the projections in another, and
//	keeps both inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-23
package schema

import (
	"sort"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ProviderModelsFrom maps a registry entry's models onto the wire shape.
func ProviderModelsFrom(entry registry.Provider) []ProviderModelResponse {
	out := make([]ProviderModelResponse, 0, len(entry.Models))
	for _, model := range entry.Models {
		out = append(out, ProviderModelResponse{
			ID:           model.ID,
			Name:         model.Name,
			Kind:         model.Kind,
			Capabilities: append([]string(nil), model.Capabilities...),
			Dimensions:   model.Dimensions,
		})
	}
	return out
}

// ProviderCountsFrom maps the repository's per-status counts onto the DTO.
func ProviderCountsFrom(counts domain.EndpointStatusCounts) ProviderStatusSummaryDTO {
	return ProviderStatusSummaryDTO{
		Total:     counts.Total,
		Active:    counts.Active,
		Disabled:  counts.Disabled,
		Error:     counts.Error,
		RateLimit: counts.RateLimited,
	}
}

// providerDisplayName prefers the display name and falls back to the id, so a
// minimal entry still renders a label instead of a blank row.
func providerDisplayName(entry registry.Provider) string {
	if entry.Display.Name != "" {
		return entry.Display.Name
	}
	return entry.ID
}

// sortMedia orders the media blocks by kind so the detail response is stable
// across reads; the registry holds them in a map, whose iteration order is
// deliberately random.
func sortMedia(media []ProviderMediaResponse) {
	sort.Slice(media, func(a, b int) bool { return media[a].Kind < media[b].Kind })
}
