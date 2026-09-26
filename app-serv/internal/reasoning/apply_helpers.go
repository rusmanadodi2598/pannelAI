// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/apply_helpers.go
// @for       The value conversions and nested-field writers the thinking formats
//
//	share.
//
// @uses      encoding/json.
// @reason    SPEC-API-001 §7.15 gives each thinking format its own field, but
//
//	several formats share the same conversions: a level to a budget, a
//	budget to a level, and the nested generationConfig Gemini keeps its
//	thinking block in. Keeping those here is what keeps apply.go to the
//	dispatch a reader compares against the reference's own table.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "encoding/json"

// commandcode params envelope.
func stripAll(body map[string]json.RawMessage) {
	for _, key := range []string{
		"thinking", "reasoning_effort", "reasoning", "thinkingConfig",
		"enable_thinking", "thinking_budget", "output_config",
	} {
		deleteField(body, key)
	}
	updateObject(body, "generationConfig", func(config map[string]json.RawMessage) {
		delete(config, "thinkingConfig")
	})
	if request, ok := objectField(body, "request"); ok {
		updateObject(request, "generationConfig", func(config map[string]json.RawMessage) {
			delete(config, "thinkingConfig")
		})
		body["request"] = mustJSON(request)
	}
	if params, ok := objectField(body, "params"); ok {
		delete(params, "reasoning_effort")
		delete(params, "thinking")
		body["params"] = mustJSON(params)
	}
}

// thinkingDisplay reads the display flag a client's thinking block carried, so
// re-applying the config does not drop it: it decides whether the upstream
// returns the thinking text at all.
func thinkingDisplay(body map[string]json.RawMessage) string {
	thinking, ok := objectField(body, "thinking")
	if !ok {
		return ""
	}
	return stringField(thinking, "display")
}

// toLevel answers the discrete level a config names, or "" when it names none.
// A budget answers its nearest level, and a budget no level covers answers
// medium, which is the reference's own fallback.
func toLevel(cfg Config) string {
	switch cfg.Mode {
	case "level":
		return cfg.Level
	case "budget":
		if level := BudgetToLevel(cfg.Budget); level != "" {
			return level
		}
		return "medium"
	case "auto":
		return "auto"
	default:
		return ""
	}
}

// toBudget answers the token budget a config names, clamped to the model's
// declared range. The second result is false when the config names no budget at
// all, which the budget formats read as their default; -1 means "the upstream's
// own default" and is never clamped, as in the reference.
func toBudget(cfg Config, caps modelCaps) (int, bool) {
	var budget int
	switch cfg.Mode {
	case "budget":
		budget = cfg.Budget
	case "level":
		value, found := EffortToBudget(cfg.Level)
		if !found {
			return 0, false
		}
		budget = value
	case "auto":
		return -1, true
	default:
		return 0, false
	}
	if caps.hasRange {
		if budget < caps.rangeMin {
			budget = caps.rangeMin
		}
		if budget > caps.rangeMax {
			budget = caps.rangeMax
		}
	}
	return budget, true
}

// zaiEffort maps a level onto the three values z.ai accepts. The reference's
// own mapping: low and minimal become low, medium and high become high, and
// everything else becomes max.
func zaiEffort(level string) string {
	switch level {
	case "low", "minimal":
		return "low"
	case "high", "medium":
		return "high"
	default:
		return "max"
	}
}

// kimiEffort maps a level onto Kimi's effort vocabulary, answering "" when the
// level has no Kimi spelling, which is the reference's null.
func kimiEffort(level string) string {
	switch level {
	case "auto":
		return "high"
	case "minimal":
		return "low"
	case "xhigh":
		return "max"
	case "low", "medium", "high", "max":
		return level
	default:
		return ""
	}
}

// setClaudeThinking writes Anthropic's thinking block, carrying the display
// flag when the client asked for one and omitting the budget for the upstream's
// own default.
func setClaudeThinking(body map[string]json.RawMessage, state string, budget int, display string) {
	thinking := map[string]any{"type": state}
	if budget > 0 {
		thinking["budget_tokens"] = budget
	}
	if display != "" {
		thinking["display"] = display
	}
	setValue(body, "thinking", thinking)
}
