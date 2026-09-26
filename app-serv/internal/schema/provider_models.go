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

// providerThinkingLevels answers the union of the levels a provider's declared
// models accept, in the reference's own order of discovery and without
// duplicates: the §7.14 picker offers this set prefixed by "auto", and "none"
// is filtered out because it is the absence of a level rather than one an
// operator picks (page.js:186-203). A provider whose models declare no
// reasoning answers nil, and the panel hides the picker then.
func providerThinkingLevels(entry registry.Provider) []string {
	levels := make([]string, 0, 4)
	seen := make(map[string]bool, 4)
	for _, model := range entry.Models {
		for _, level := range thinkingLevelsFor(entry.ID, model.ID) {
			if seen[level] {
				continue
			}
			seen[level] = true
			levels = append(levels, level)
		}
	}
	if len(levels) == 0 {
		return nil
	}
	return levels
}

// thinkingLevelsFor answers one model's pickable levels: the registry's own set
// with "none" removed, because a picker offers choices and "none" is the
// absence of one (the reference's own filter, thinkingLevels.js:70-73). An id
// the registry does not know answers nil, which is what keeps a suffix off a
// model whose upstream would refuse it.
func thinkingLevelsFor(providerID, modelID string) []string {
	levels := registry.ThinkingLevels(providerID, modelID)
	pickable := make([]string, 0, len(levels))
	for _, level := range levels {
		if level != "none" {
			pickable = append(pickable, level)
		}
	}
	if len(pickable) == 0 {
		return nil
	}
	return pickable
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
