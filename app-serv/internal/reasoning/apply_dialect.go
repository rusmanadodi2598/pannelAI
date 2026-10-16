// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/apply_dialect.go
// @for       The vendor-shaped thinking fields: the nine bodies that name their own switch, budget, or effort enum instead of the wire's.
// @uses      encoding/json.
// @reason    A stored per-provider mode reaches every model on that provider, so each dialect's own accepted vocabulary has to be written where it is known: these shapes are the reference's per-vendor branches (thinkingUnified.js:290-355) and each carries the refusal it was measured against. They sit apart from apply.go because the wire-native shapes and the vendor ones are two lists that change for different reasons (AGENTS.md §1.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-16
package reasoning

import "encoding/json"

// applyDialectFormat writes one vendor's own thinking shape. The caller has
// already resolved the mode and the model's clamped config; `none` is the
// original request to disable, which a dialect that cannot switch thinking off
// reads as its lowest state.
func applyDialectFormat(body map[string]json.RawMessage, format string, eff Config, none bool, caps modelCaps) {
	switch format {
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

	case "tokenrouter":
		// TokenRouter's reasoning_effort enum is low/medium/high/xhigh/max: it
		// rejects "none" and "auto" with a 400 and reads "max" natively, so the
		// field is omitted rather than clamped and every level passes through
		// as it stands. The guard reads the original mode, not the clamped
		// one: a model that cannot disable would otherwise receive the
		// "minimal" the enum does not list.
		if none || eff.Mode == "auto" {
			return
		}
		if level := toLevel(eff); level != "" {
			setValue(body, "reasoning_effort", level)
		}

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
