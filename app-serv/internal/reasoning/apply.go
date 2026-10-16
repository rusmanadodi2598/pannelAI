// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/apply.go
// @for       Writing one resolved intent into the outbound body in the wire shape its provider's format reads.
// @uses      encoding/json.
// @reason    SPEC-API-001 §7.15 ports the reference's thinkingUnified.js applyFormat: thirteen provider-native shapes, each the field that upstream actually reads. A field the upstream ignores is a control that looks wired and does nothing, so each format's own dispatch is carried rather than one generic field.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
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
// Claude formats. Whichever source named the level, the two OpenAI shapes ask it
// against the model's own list, because the list is what that vendor declares the
// model reads and a level outside it is a control nobody documented.
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
			setValue(body, "reasoning_effort", ReasoningLevelFor(level, caps.levels))
		}

	case "openai-responses":
		// The Responses API reads reasoning:{effort,summary}, not the chat
		// field: the reference's opencode and codex executors rewrite the
		// effort into this object and force the summary on, because a
		// Responses upstream reasons anyway but streams no summary text a
		// client could carry until the request asks for one
		// (executors/opencode.js normalizeOpencodeReasoning, codex.js:459).
		if none && caps.canDisable {
			setValue(body, "reasoning", map[string]any{"effort": "none", "summary": "auto"})
			return
		}
		if level := toLevel(eff); level != "" {
			setValue(body, "reasoning", map[string]any{
				"effort":  ReasoningLevelFor(level, caps.levels),
				"summary": "auto",
			})
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

	case "zai", "qwen", "deepseek", "kimi", "minimax", "hunyuan", "step", "tokenrouter", "commandcode":
		applyDialectFormat(body, format, eff, none, caps)
	}
}
