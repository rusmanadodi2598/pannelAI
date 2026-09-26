// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/apply_vendor_test.go
// @for       The vendor dialects an OpenAI-wire provider declares for itself:
//
//	zai, qwen, deepseek, kimi, minimax, hunyuan, step, and commandcode.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.15 ports thinkingUnified.js applyFormat, and these
//
//	eight formats are the ones that do not speak the plain OpenAI effort
//	field: each renames it, wraps it in an envelope, or pairs it with its
//	own switch, so each needs its own proof that the member it writes is
//	the member its upstream reads. The gateway's own three wires are
//	proven in apply_wire_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "testing"

// TestApplyFormat_VendorDialects pins the field each vendor dialect writes,
// including the two switches that turn the effort into an enum and the
// envelope commandcode keeps its parameters in.
func TestApplyFormat_VendorDialects(t *testing.T) {
	runApplyCases(t, []applyCase{
		{
			name: "zai writes the switch and the effort", body: `{}`, format: "zai",
			cfg:  Config{Mode: "level", Level: "medium"},
			caps: modelCaps{canDisable: true, effort: true},
			want: `{"thinking":{"type":"enabled"},"reasoning_effort":"high"}`,
		},
		{
			name: "zai omits the effort on an older model", body: `{}`, format: "zai",
			cfg:  Config{Mode: "level", Level: "medium"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"enabled"}}`,
		},
		{
			name: "zai disables with the flag", body: `{}`, format: "zai",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"enable_thinking":false}`,
		},
		{
			name: "qwen writes the flag and the budget", body: `{}`, format: "qwen",
			cfg:  Config{Mode: "level", Level: "medium"},
			caps: modelCaps{canDisable: true},
			want: `{"enable_thinking":true,"thinking_budget":8192}`,
		},
		{
			name: "qwen none writes only the flag", body: `{}`, format: "qwen",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"enable_thinking":false}`,
		},
		{
			name: "deepseek writes the switch and a clamped effort", body: `{}`, format: "deepseek",
			cfg:  Config{Mode: "level", Level: "low"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"enabled"},"reasoning_effort":"high"}`,
		},
		{
			name: "deepseek maps the top levels to max", body: `{}`, format: "deepseek",
			cfg:  Config{Mode: "level", Level: "max"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"enabled"},"reasoning_effort":"max"}`,
		},
		{
			name: "deepseek none disables", body: `{}`, format: "deepseek",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"disabled"}}`,
		},
		{
			name: "kimi maps auto to high", body: `{}`, format: "kimi",
			cfg:  Config{Mode: "auto"},
			caps: modelCaps{canDisable: true},
			want: `{"reasoning_effort":"high"}`,
		},
		{
			name: "kimi maps xhigh to max", body: `{}`, format: "kimi",
			cfg:  Config{Mode: "level", Level: "xhigh"},
			caps: modelCaps{canDisable: true},
			want: `{"reasoning_effort":"max"}`,
		},
		{
			name: "minimax writes the adaptive switch", body: `{}`, format: "minimax",
			cfg:  Config{Mode: "level", Level: "high"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"adaptive"}}`,
		},
		{
			name: "minimax none disables", body: `{}`, format: "minimax",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"disabled"}}`,
		},
		{
			name: "hunyuan writes the budget", body: `{}`, format: "hunyuan",
			cfg:  Config{Mode: "budget", Budget: 1024},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"enabled","budget_tokens":1024}}`,
		},
		{
			name: "hunyuan auto leaves the budget to the upstream", body: `{}`, format: "hunyuan",
			cfg:  Config{Mode: "auto"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"enabled"}}`,
		},
		{
			name: "step writes the effort", body: `{}`, format: "step",
			cfg:  Config{Mode: "level", Level: "medium"},
			caps: modelCaps{canDisable: true},
			want: `{"reasoning_effort":"medium"}`,
		},
		{
			name: "step clamps the top levels", body: `{}`, format: "step",
			cfg:  Config{Mode: "level", Level: "max"},
			caps: modelCaps{canDisable: true},
			want: `{"reasoning_effort":"high"}`,
		},
		{
			name: "step none writes nothing", body: `{}`, format: "step",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{}`,
		},
		{
			name: "commandcode writes inside the params envelope", body: `{"params":{"temperature":0.2}}`, format: "commandcode",
			cfg:  Config{Mode: "level", Level: "high"},
			caps: modelCaps{canDisable: true},
			want: `{"params":{"temperature":0.2,"reasoning_effort":"high"}}`,
		},
		{
			name: "commandcode none removes the effort", body: `{"params":{"reasoning_effort":"high"}}`, format: "commandcode",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"params":{}}`,
		},
	})
}
