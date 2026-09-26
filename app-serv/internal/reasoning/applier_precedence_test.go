// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/applier_precedence_test.go
// @for       Which source wins: the model string's suffix, the client's own
//
//	body, or the stored provider mode, per model family.
//
// @uses      context, testing, internal/domain.
// @reason    SPEC-API-001 §7.14 stores one mode per provider and §7.15 carries
//
//	the suffix, so the precedence is the seam's whole contract and a
//	wrong answer is silent: a body that gained a field the client did
//	not ask for, or lost one it did. The table is one file because it
//	is one question, and applier_test.go keeps the cases that must
//	leave a body untouched (AGENTS.md §1.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package reasoning

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestApplier_ResolvesThePrecedence pins the reference's order: the suffix
// outranks the client's own intent, which outranks the stored provider mode.
func TestApplier_ResolvesThePrecedence(t *testing.T) {
	override := &Suffix{Mode: "level", Level: "max"}
	cases := []struct {
		name     string
		settings domain.Settings
		call     Call
		want     string
	}{
		{
			name:     "the stored mode is written when the client asked for nothing",
			settings: settingsWithThinking("openai", "high"),
			call:     Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5"},
			want:     `{"model":"m","reasoning_effort":"high"}`,
		},
		{
			name:     "the client's effort outranks the stored mode",
			settings: settingsWithThinking("openai", "high"),
			call: Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5",
				ClientRaw: []byte(`{"reasoning_effort":"low"}`)},
			want: `{"model":"m","reasoning_effort":"low"}`,
		},
		{
			name:     "the suffix outranks both, clamped to the model's list",
			settings: settingsWithThinking("openai", "high"),
			call: Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5",
				ClientRaw: []byte(`{"reasoning_effort":"low"}`), Override: override},
			// The openai format does not list max for this model, so the
			// reference's own clamp answers xhigh.
			want: `{"model":"m","reasoning_effort":"xhigh"}`,
		},
		{
			name:     "a stored on is the reference's fixed budget",
			settings: settingsWithThinking("openai", domain.ThinkingOn),
			call:     Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5"},
			want:     `{"model":"m","reasoning_effort":"medium"}`,
		},
		{
			name:     "a stored off disables",
			settings: settingsWithThinking("openai", domain.ThinkingOff),
			call:     Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5"},
			want:     `{"model":"m","reasoning_effort":"none"}`,
		},
		{
			name:     "a model that cannot disable thinking clamps off",
			settings: settingsWithThinking("glm", domain.ThinkingOff),
			call:     Call{Wire: "openai", ProviderID: "glm", ModelID: "glm-5.2"},
			want:     `{"model":"m","thinking":{"type":"enabled"}}`,
		},
		{
			name:     "a Claude-format model gets the adaptive shape",
			settings: settingsWithThinking("anthropic", "low"),
			call:     Call{Wire: "claude", ProviderID: "anthropic", ModelID: "claude-opus-4-8"},
			want:     `{"model":"m","thinking":{"type":"adaptive"},"output_config":{"effort":"low"}}`,
		},
		{
			name:     "a suffix budget on a Claude-format model becomes a level",
			settings: settingsWithThinking("anthropic", "high"),
			call: Call{Wire: "claude", ProviderID: "anthropic", ModelID: "claude-opus-4-8",
				Override: &Suffix{Mode: "budget", Budget: 4096}},
			want: `{"model":"m","thinking":{"type":"adaptive"},"output_config":{"effort":"low"}}`,
		},
		{
			name:     "a deepseek model writes its switch and effort",
			settings: settingsWithThinking("deepseek", "high"),
			call:     Call{Wire: "openai", ProviderID: "deepseek", ModelID: "deepseek-reasoner"},
			want:     `{"model":"m","thinking":{"type":"enabled"},"reasoning_effort":"high"}`,
		},
		{
			name:     "a commandcode model writes inside its params envelope",
			settings: settingsWithThinking("commandcode", "high"),
			call:     Call{Wire: "openai", ProviderID: "commandcode", ModelID: "MiniMaxAI/MiniMax-M3"},
			want:     `{"model":"m","params":{"reasoning_effort":"high"}}`,
		},
		{
			name:     "a gemini-format model on the OpenAI wire falls back to the wire's shape",
			settings: settingsWithThinking("opencode-zen", "low"),
			call:     Call{Wire: "openai", ProviderID: "opencode-zen", ModelID: "gemini-3-flash"},
			want:     `{"model":"m","reasoning_effort":"low"}`,
		},
		{
			name:     "a kimi model maps the level to its own vocabulary",
			settings: settingsWithThinking("kimi", "xhigh"),
			call:     Call{Wire: "openai", ProviderID: "kimi", ModelID: "kimi-k3"},
			want:     `{"model":"m","reasoning_effort":"max"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			applier, err := NewApplier(stubSettings{settings: tc.settings})
			if err != nil {
				t.Fatalf("NewApplier() error = %v", err)
			}
			got := applier.Apply(context.Background(), []byte(`{"model":"m"}`), tc.call)
			assertJSON(t, decodeAny(t, got), tc.want)
		})
	}
}
