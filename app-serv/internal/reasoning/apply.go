// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/apply.go
// @for       Writing one resolved intent into the outbound body in the wire
//
//	shape its provider's format reads.
//
// @uses      encoding/json.
// @reason    SPEC-API-001 §7.15 ports the reference's thinkingUnified.js
//
//	applyFormat: thirteen provider-native shapes, each the field that
//	upstream actually reads. A field the upstream ignores is a control
//	that looks wired and does nothing, so each format's own dispatch is
//	carried rather than one generic field.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "encoding/json"

// OnBudget is the fixed budget the reference's "on" mode means: its injection
// writes thinking{type:"enabled", budget_tokens:10000} (chatCore.js:111), and
// this port resolves the stored "on" to the same number.
const OnBudget = 10000

// DefaultBudget is the fallback a budget format uses when the intent names no
// resolvable budget, which is the reference's `budget || 8192`.
const DefaultBudget = 8192

// modelCaps is the slice of a model's capability answer the application reads.
type modelCaps struct {
	canDisable bool
	hasRange   bool
	rangeMin   int
	rangeMax   int
	effort     bool
	levels     []string
}

// applyFormat writes cfg into body in the named format's native shape. It is a
// port of the reference's applyFormat (thinkingUnified.js:233-355): the model's
// own format decides the field, a model that cannot disable thinking clamps
// "none" to the lowest level, and the optional display flag survives for the
// Claude formats.
func applyFormat(body map[string]json.RawMessage, format string, cfg Config, caps modelCaps, display string) {
	none := cfg.Mode == "none"
	eff := cfg
	if none && !caps.canDisable {
		eff = Config{Mode: "level", Level: "minimal"}
	}

	switch format {
	case "openai":
		if none && caps.canDisable {
			setValue(body, "reasoning_effort", "none")
			return
		}
		if level := toLevel(eff); level != "" {
			setValue(body, "reasoning_effort", NormalizeOpenAILevel(level, caps.levels))
		}

	case "claude-adaptive":
		if none && caps.canDisable {
			setClaudeThinking(body, "disabled", 0, display)
			return
		}
		// A model that can disable thinking needs the explicit switch; a
		// permanently adaptive one takes the effort directly.
		if caps.canDisable {
			setClaudeThinking(body, "adaptive", 0, display)
		} else {
			deleteField(body, "thinking")
		}
		level := toLevel(eff)
		if level == "xhigh" || level == "auto" {
			level = "high"
		}
		setValue(body, "output_config", map[string]any{"effort": level})

	case "claude-budget":
		if none && caps.canDisable {
			setClaudeThinking(body, "disabled", 0, display)
			return
		}
		budget, found := toBudget(eff, caps)
		if !found || budget == 0 {
			budget = DefaultBudget
		}
		if budget == -1 {
			setClaudeThinking(body, "enabled", 0, display)
			return
		}
		setClaudeThinking(body, "enabled", budget, display)

	case "gemini-level":
		level := "minimal"
		if !none {
			level = EffortToThinkingLevel(toLevel(eff))
		}
		setGeminiThinking(body, map[string]any{"thinkingLevel": level, "includeThoughts": level != "minimal"})
		ensureGeminiOutputFloor(body, geminiLevelOutputFloor(level))

	case "gemini-budget":
		if none && caps.canDisable {
			setGeminiThinking(body, map[string]any{"thinkingBudget": 0, "includeThoughts": false})
			return
		}
		budget, found := toBudget(eff, caps)
		if !found {
			budget = -1
		}
		setGeminiThinking(body, map[string]any{"thinkingBudget": budget, "includeThoughts": true})
		ensureGeminiOutputFloor(body, geminiBudgetOutputFloor(budget))

	case "zai":
		// z.ai ignores thinking.disabled, so the flag is what turns it off.
		if none && caps.canDisable {
			setValue(body, "enable_thinking", false)
			deleteField(body, "thinking")
			return
		}
		setValue(body, "thinking", map[string]any{"type": "enabled"})
		// Only the models from GLM-5.2 onward read reasoning_effort; an older
		// GLM would receive a field it does not know.
		if caps.effort {
			setValue(body, "reasoning_effort", zaiEffort(toLevel(eff)))
		}

	case "qwen":
		if none && caps.canDisable {
			setValue(body, "enable_thinking", false)
			return
		}
		setValue(body, "enable_thinking", true)
		if budget, found := toBudget(eff, caps); found && budget > 0 {
			setValue(body, "thinking_budget", budget)
		}

	case "deepseek":
		if none && caps.canDisable {
			setValue(body, "thinking", map[string]any{"type": "disabled"})
			return
		}
		setValue(body, "thinking", map[string]any{"type": "enabled"})
		level := toLevel(eff)
		if level == "xhigh" || level == "max" {
			setValue(body, "reasoning_effort", "max")
			return
		}
		setValue(body, "reasoning_effort", "high")

	case "kimi":
		if none && caps.canDisable {
			setValue(body, "thinking", map[string]any{"type": "disabled"})
			return
		}
		if effort := kimiEffort(toLevel(eff)); effort != "" {
			setValue(body, "reasoning_effort", effort)
		}

	case "minimax":
		state := "adaptive"
		if none && caps.canDisable {
			state = "disabled"
		}
		setValue(body, "thinking", map[string]any{"type": state})

	case "hunyuan":
		if none && caps.canDisable {
			setValue(body, "thinking", map[string]any{"type": "disabled"})
			return
		}
		budget, found := toBudget(eff, caps)
		if !found || budget == 0 {
			budget = DefaultBudget
		}
		if budget == -1 {
			setValue(body, "thinking", map[string]any{"type": "enabled"})
			return
		}
		setValue(body, "thinking", map[string]any{"type": "enabled", "budget_tokens": budget})

	case "step":
		if none && caps.canDisable {
			return
		}
		level := toLevel(eff)
		if level == "" {
			return
		}
		if level == "xhigh" || level == "max" {
			level = "high"
		}
		setValue(body, "reasoning_effort", level)

	case "commandcode":
		params := ensureObject(body, "params")
		if none && caps.canDisable {
			delete(params, "reasoning_effort")
			body["params"] = mustJSON(params)
			return
		}
		if level := toLevel(eff); level != "" {
			params["reasoning_effort"] = mustJSON(level)
		}
		body["params"] = mustJSON(params)
	}
}

// stripAll removes every thinking field this port knows, so a re-applied config
// cannot leave a stale member from the client's own shape beside the new one.
// It is the reference's stripAll, including the nested generationConfig and the
