// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/apply_wire_test.go
// @for       The formats the gateway's own three wires speak: OpenAI, the two
//
//	Claude shapes, and the two Gemini shapes.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.15 ports thinkingUnified.js applyFormat, and the
//
//	whole point of the port is that each upstream reads its own field: a
//	format that writes the wrong member is a control that looks wired and
//	does nothing. These five formats are the ones a wire target maps onto
//	directly (apply.go's resolveFormat), which is why they are proven
//	together; the vendor dialects live in apply_vendor_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "testing"

// TestApplyFormat_WireFamilies pins the field each of the gateway's own wire
// formats writes, including the two clamps: a model that cannot disable
// thinking, and a budget range.
func TestApplyFormat_WireFamilies(t *testing.T) {
	runApplyCases(t, []applyCase{
		{
			name: "openai writes the effort", body: `{}`, format: "openai",
			cfg:  Config{Mode: "level", Level: "high"},
			caps: modelCaps{canDisable: true},
			want: `{"reasoning_effort":"high"}`,
		},
		{
			name: "openai none disables", body: `{}`, format: "openai",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"reasoning_effort":"none"}`,
		},
		{
			name: "openai none on a model that cannot disable clamps", body: `{}`, format: "openai",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: false},
			want: `{"reasoning_effort":"minimal"}`,
		},
		{
			name: "openai max clamps to the model's list", body: `{}`, format: "openai",
			cfg:  Config{Mode: "level", Level: "max"},
			caps: modelCaps{canDisable: true, levels: []string{"low", "high", "xhigh"}},
			want: `{"reasoning_effort":"xhigh"}`,
		},
		{
			name: "claude-adaptive writes the switch and the effort", body: `{}`, format: "claude-adaptive",
			cfg:  Config{Mode: "level", Level: "high"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"adaptive"},"output_config":{"effort":"high"}}`,
		},
		{
			name: "claude-adaptive none disables", body: `{}`, format: "claude-adaptive",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"disabled"}}`,
		},
		{
			name: "claude-adaptive clamps xhigh to high", body: `{}`, format: "claude-adaptive",
			cfg:  Config{Mode: "level", Level: "xhigh"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"adaptive"},"output_config":{"effort":"high"}}`,
		},
		{
			name: "claude-adaptive keeps the client's display", body: `{}`, format: "claude-adaptive",
			cfg: Config{Mode: "level", Level: "low"}, caps: modelCaps{canDisable: true}, display: "summarized",
			want: `{"thinking":{"type":"adaptive","display":"summarized"},"output_config":{"effort":"low"}}`,
		},
		{
			name: "claude-budget writes the budget", body: `{}`, format: "claude-budget",
			cfg:  Config{Mode: "budget", Budget: 8192},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"enabled","budget_tokens":8192}}`,
		},
		{
			name: "claude-budget auto leaves the budget to the upstream", body: `{}`, format: "claude-budget",
			cfg:  Config{Mode: "auto"},
			caps: modelCaps{canDisable: true},
			want: `{"thinking":{"type":"enabled"}}`,
		},
		{
			name: "claude-budget clamps to the model's range", body: `{}`, format: "claude-budget",
			cfg:  Config{Mode: "budget", Budget: 99999},
			caps: modelCaps{canDisable: true, hasRange: true, rangeMin: 0, rangeMax: 24576},
			want: `{"thinking":{"type":"enabled","budget_tokens":24576}}`,
		},
		{
			name: "claude-budget clamps none to the lowest level", body: `{}`, format: "claude-budget",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: false},
			want: `{"thinking":{"type":"enabled","budget_tokens":512}}`,
		},
		{
			name: "gemini-level writes the level and raises the floor", body: `{}`, format: "gemini-level",
			cfg:  Config{Mode: "level", Level: "high"},
			caps: modelCaps{canDisable: true},
			want: `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high","includeThoughts":true},"maxOutputTokens":65535}}`,
		},
		{
			name: "gemini-level none becomes minimal", body: `{}`, format: "gemini-level",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"minimal","includeThoughts":false},"maxOutputTokens":4096}}`,
		},
		{
			name: "gemini-budget writes the budget", body: `{}`, format: "gemini-budget",
			cfg:  Config{Mode: "budget", Budget: 5000},
			caps: modelCaps{canDisable: true},
			want: `{"generationConfig":{"thinkingConfig":{"thinkingBudget":5000,"includeThoughts":true},"maxOutputTokens":16384}}`,
		},
		{
			name: "gemini-budget none disables without raising the floor", body: `{}`, format: "gemini-budget",
			cfg:  Config{Mode: "none"},
			caps: modelCaps{canDisable: true},
			want: `{"generationConfig":{"thinkingConfig":{"thinkingBudget":0,"includeThoughts":false}}}`,
		},
		{
			name: "gemini-budget auto leaves the budget to the upstream", body: `{}`, format: "gemini-budget",
			cfg:  Config{Mode: "auto"},
			caps: modelCaps{canDisable: true},
			want: `{"generationConfig":{"thinkingConfig":{"thinkingBudget":-1,"includeThoughts":true},"maxOutputTokens":32768}}`,
		},
	})
}
