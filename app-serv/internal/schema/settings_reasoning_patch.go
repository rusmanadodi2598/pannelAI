// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_reasoning_patch.go
// @for       The reasoning group's PATCH DTO, its lowering into the domain
//
//	mutation, and the per-entry rules of the provider mode map.
//
// @uses      internal/domain.
// @reason    SPEC-API-001 §7.14 validates a PATCH per key. The provider mode
//
//	map is a whole replacement whose entries carry their own vocabulary,
//	so keeping its DTO and checks together is what stops the settings
//	patch file from growing past the line budget while the group stays
//	one reviewable unit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-26
package schema

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"

// ReasoningSettingsPatch updates the reasoning group. The provider map is a
// whole replacement, so an empty object clears every entry and "auto" is
// expressed by deleting the entry rather than by storing a mode. The oneof
// values are pinned against domain.ThinkingModes by a test, so the tag and the
// injection cannot drift apart.
type ReasoningSettingsPatch struct {
	ProviderThinking *map[string]ProviderThinkingPatch `json:"provider_thinking,omitempty" validate:"omitempty,dive"`
}

// ProviderThinkingPatch is one provider's stored reasoning mode.
type ProviderThinkingPatch struct {
	Mode *string `json:"mode,omitempty" validate:"omitempty,oneof=on off none minimal low medium high xhigh max thinking"`
}

// validateProviderThinking applies the entry rule the struct tags cannot
// express: every entry must name a mode, because an entry without one is
// configuration nobody reads. An empty string is refused here rather than left
// to the domain backstop, because `omitempty` lets it past the oneof tag. The
// reference's panel deletes the entry instead of writing an empty one, which is
// the same rule expressed in its own way.
func validateProviderThinking(in *ReasoningSettingsPatch) error {
	if in == nil || in.ProviderThinking == nil {
		return nil
	}
	for id, entry := range *in.ProviderThinking {
		if entry.Mode == nil || *entry.Mode == "" {
			return domain.NewValidationError("provider_thinking[" + id +
				"].mode is required; delete the entry to follow the client")
		}
	}
	return nil
}

// reasoningPtr lowers the reasoning group into the domain mutation object. The
// DTO layer has already rejected an out-of-set or empty mode, so an unstorable
// value cannot reach here.
func reasoningPtr(in *ReasoningSettingsPatch) *domain.ReasoningSettingsPatch {
	if in == nil {
		return nil
	}
	patch := &domain.ReasoningSettingsPatch{}
	if in.ProviderThinking != nil {
		replaced := make(map[string]domain.ProviderThinking, len(*in.ProviderThinking))
		for id, entry := range *in.ProviderThinking {
			if entry.Mode != nil {
				replaced[id] = domain.ProviderThinking{Mode: domain.ThinkingMode(*entry.Mode)}
			}
		}
		patch.ProviderThinking = &replaced
	}
	return patch
}

// thinkingOrEmpty renders the mode map, never as null, so a panel round-trip
// cannot turn "no entries" into a missing member.
func thinkingOrEmpty(in map[string]domain.ProviderThinking) map[string]ProviderThinkingResponse {
	out := make(map[string]ProviderThinkingResponse, len(in))
	for id, entry := range in {
		out[id] = ProviderThinkingResponse{Mode: string(entry.Mode)}
	}
	return out
}
