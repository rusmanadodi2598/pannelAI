// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_reasoning.go
// @for       The reasoning group: the stored thinking-mode vocabulary, its
//
//	per-provider resolution, and the patch rules.
//
// @uses      strings.
// @reason    SPEC-API-001 §7.14 gains a reasoning group for the reference's
//
//	providerThinking map: one mode per provider, applied to the outbound
//	body when the client carries no reasoning intent of its own. Keeping
//	the vocabulary here, beside the resolution rule, is what stops a
//	stored mode the injection cannot execute from being written.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-26
package domain

import "strings"

// ThinkingMode is one provider's stored reasoning mode: an extended-thinking
// state ("on" carries a fixed budget, "off" disables) or a level word the
// reference's per-format sets accept. "auto" is deliberately not a value: it
// is the absence of the entry, which the panel expresses by deleting it.
type ThinkingMode string

const (
	// ThinkingOn forces extended thinking with the reference's fixed budget
	// when the client has not set a thinking object of its own.
	ThinkingOn ThinkingMode = "on"
	// ThinkingOff disables extended thinking unless the client asked for it.
	ThinkingOff ThinkingMode = "off"
)

// ThinkingModes is every mode a stored entry may name, in the order the
// contract documents them: the two states, then the union of the level words
// the reference's formats accept in escalation order. The schema's oneof tag
// is pinned against this list by a test.
var ThinkingModes = []string{
	"on", "off",
	"none", "minimal", "low", "medium", "high", "xhigh", "max", "thinking",
}

// ValidThinkingMode reports whether a mode is one the injection can execute.
func ValidThinkingMode(mode ThinkingMode) bool {
	for _, known := range ThinkingModes {
		if ThinkingMode(known) == mode {
			return true
		}
	}
	return false
}

// ProviderThinking is one provider's stored reasoning mode.
type ProviderThinking struct {
	Mode ThinkingMode `json:"mode"`
}

// ReasoningSettings is the §7.14 reasoning group. ProviderThinking is keyed by
// provider id and written whole, like the routing group's override map: the
// panel reads it, changes one entry, and sends the map back.
type ReasoningSettings struct {
	ProviderThinking map[string]ProviderThinking `json:"provider_thinking,omitempty"`
}

// defaultReasoningSettings is the §7.14 default: no provider carries a mode,
// so every request keeps the reasoning intent its client sent.
func defaultReasoningSettings() ReasoningSettings {
	return ReasoningSettings{ProviderThinking: map[string]ProviderThinking{}}
}

// Normalized fills the map a stored row written before this group existed
// cannot carry, so a read answers an object rather than null.
func (s ReasoningSettings) Normalized() ReasoningSettings {
	if s.ProviderThinking == nil {
		s.ProviderThinking = map[string]ProviderThinking{}
	}
	return s
}

// ThinkingFor resolves one provider's stored mode. The second result is false
// when the provider has no entry or carries a mode the injection cannot
// execute, which both mean the same thing to the data plane: leave the
// client's own reasoning intent alone.
func (s ReasoningSettings) ThinkingFor(providerID string) (ThinkingMode, bool) {
	entry, found := s.ProviderThinking[providerID]
	if !found || !ValidThinkingMode(entry.Mode) {
		return "", false
	}
	return entry.Mode, true
}

// ReasoningSettingsPatch updates the reasoning group. ProviderThinking
// replaces the whole map when present, so an empty map is a real value (clear
// every provider) rather than "leave unchanged", which the nil pointer already
// means.
type ReasoningSettingsPatch struct {
	ProviderThinking *map[string]ProviderThinking
}

// applyReasoning writes the reasoning group from a patch.
func applyReasoning(dst *ReasoningSettings, p *ReasoningSettingsPatch) {
	if p.ProviderThinking == nil {
		return
	}
	replaced := make(map[string]ProviderThinking, len(*p.ProviderThinking))
	for id, entry := range *p.ProviderThinking {
		replaced[id] = entry
	}
	dst.ProviderThinking = replaced
}

// validateReasoning checks the group's entry rule: every stored mode must be
// one the injection can execute, because an entry nobody can act on is dead
// configuration. "auto" is refused with the advice the reference's panel
// already follows: delete the entry.
func validateReasoning(s ReasoningSettings) error {
	for id, entry := range s.ProviderThinking {
		if !ValidThinkingMode(entry.Mode) {
			return NewValidationError("reasoning.provider_thinking[" + id +
				"].mode must be one of " + strings.Join(ThinkingModes, ", ") +
				"; delete the entry to follow the client")
		}
	}
	return nil
}
