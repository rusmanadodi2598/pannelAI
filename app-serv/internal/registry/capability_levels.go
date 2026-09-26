// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_levels.go
// @for       The thinking levels a model accepts, ported from the reference's
//
//	thinkingLevels.js: the per-format sets, the model-name overrides, and
//	the can-disable filter.
//
// @uses      strings.
// @reason    The reasoning control (§7.14) offers one list of modes per model,
//
//	and the reference computes that list from the same capability
//	resolution capability_thinking.go carries (thinkingLevels.js:66-76).
//	The sets are the reference's own, verified against its wire dispatch,
//	because a level a model does not accept is a control that answers an
//	upstream error instead of a choice.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package registry

import "strings"

// baseLevels is the reference's shared set for the budget-style formats (qwen,
// step, hunyuan, gemini-budget).
var baseLevels = []string{"none", "low", "medium", "high"}

// thinkingFormatLevels maps a thinking format to the levels it accepts. It is
// the reference's FORMAT_LEVELS table, with the shared sets spelled once.
var thinkingFormatLevels = map[string][]string{
	"openai":          {"none", "minimal", "low", "medium", "high", "xhigh"},
	"claude-adaptive": {"none", "low", "medium", "high", "max"},
	"claude-budget":   {"none", "low", "medium", "high", "xhigh", "max"},
	"gemini-level":    {"minimal", "low", "medium", "high"},
	"gemini-budget":   baseLevels,
	"zai":             {"none", "thinking"},
	"qwen":            baseLevels,
	"kimi":            {"none", "low", "medium", "high", "max"},
	"deepseek":        {"none", "high", "max"},
	"commandcode":     {"none", "low", "medium", "high", "xhigh", "max"},
	"minimax":         {"none", "thinking"},
	"hunyuan":         baseLevels,
	"step":            baseLevels,
}

// thinkingLevelRule is one model-name override: a glob and the levels it fixes,
// optionally scoped to one provider. The reference's rows for the codex provider
// are dropped because the owner's KEEP set (2026-09-26) removed it, so no model
// can reach them; every other row is carried, including the provider-scoped
// codebuddy rows, because those change the answer for models the registry
// declares.
type thinkingLevelRule struct {
	provider string
	pattern  string
	levels   []string
}

// thinkingLevelRules is the reference's PATTERN_THINKING table in its order: the
// first match wins, and a provider-scoped row only matches its provider.
var thinkingLevelRules = []thinkingLevelRule{
	{pattern: "*codex*", levels: []string{"low", "medium", "high", "xhigh"}}, // codex cannot disable thinking
	{pattern: "*mimo*v2.6*", levels: []string{"none", "low", "medium", "high", "xhigh"}},
	{pattern: "*deepseek-v4.*", levels: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	{provider: "codebuddy-cn", pattern: "glm-5.3*", levels: []string{"low", "high", "max"}},
	{provider: "codebuddy-cn", pattern: "glm-5.2", levels: []string{"high", "xhigh"}},
	{provider: "codebuddy-cn", pattern: "deepseek-v4*", levels: []string{"low", "high", "xhigh"}},
	{provider: "codebuddy-cn", pattern: "hy3*", levels: []string{"low", "high"}},
	{provider: "codebuddy-cn", pattern: "hy4*", levels: []string{"high"}},
	{provider: "codebuddy-intl", pattern: "deepseek-v4*", levels: []string{"low", "high", "xhigh"}},
}

// ThinkingLevels answers the levels a model accepts, or nil when it does not
// reason. The reference's own function (thinkingLevels.js:66-76): the capability
// resolution decides whether the model reasons and which format it uses, a
// model-name override may replace the format's set, and a model that cannot
// disable thinking loses "none" because the level would be clamped upstream.
//
// The list keeps the reference's spelling, including "none"; a caller that
// builds a picker filters it, because a picker offers choices rather than
// states.
func ThinkingLevels(provider, modelID string) []string {
	caps := Capabilities(provider, modelID)
	if !caps.Reasoning {
		return nil
	}
	id := strings.ToLower(strings.TrimSpace(modelID))
	base := id
	if slash := strings.LastIndex(id, "/"); slash >= 0 {
		base = id[slash+1:]
	}

	levels := thinkingFormatLevels[caps.ThinkingFormat]
	if len(levels) == 0 {
		levels = baseLevels
	}
	for _, rule := range thinkingLevelRules {
		if rule.provider != "" && !strings.EqualFold(rule.provider, strings.TrimSpace(provider)) {
			continue
		}
		if matchesGlob(rule.pattern, base) || matchesGlob(rule.pattern, id) {
			levels = rule.levels
			break
		}
	}
	if caps.CanDisable {
		return levels
	}
	filtered := make([]string, 0, len(levels))
	for _, level := range levels {
		if level != "none" {
			filtered = append(filtered, level)
		}
	}
	return filtered
}
