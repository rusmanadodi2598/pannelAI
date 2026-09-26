// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_thinking_test.go
// @for       Tests for the reasoning tables and the thinking-level sets: the
//
//	layer boundaries, the shadowing rows, and the can-disable filter.
//
// @uses      testing.
// @reason    The corpus test pins every model the registry declares, but it
//
//	cannot say which layer answered: a resolver that dropped the provider
//	overrides would still pass on rows the pattern table happens to answer
//	the same way. These tests name the boundaries, so the layer that goes
//	missing fails by name instead of as a silent fall-through.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package registry

import "testing"

// TestThinkingFor_Layers pins one id per resolution layer, including the
// commandcode branch and the floor.
func TestThinkingFor_Layers(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		model    string
		want     thinkingAnswer
	}{
		{
			name: "the commandcode branch answers before any table", provider: "commandcode", model: "deepseek/deepseek-v4-pro",
			want: thinkingAnswer{Reasoning: true, Format: "commandcode", CanDisable: true},
		},
		{
			name: "a provider override beats the exact and pattern layers", provider: "nvidia", model: "z-ai/glm-5.2",
			want: thinkingAnswer{Reasoning: true, Format: "openai", CanDisable: true},
		},
		{
			name: "a provider override can also forbid disabling", provider: "codebuddy-cn", model: "kimi-k2.6",
			want: thinkingAnswer{Reasoning: true, Format: "openai", CanDisable: false},
		},
		{
			name: "the exact layer splits adaptive from budget", provider: "claude", model: "claude-opus-4-7",
			want: thinkingAnswer{Reasoning: true, Format: "claude-adaptive", CanDisable: true},
		},
		{
			name: "the pattern layer carries the family format", provider: "glm", model: "glm-4.6",
			want: thinkingAnswer{Reasoning: true, Format: "zai", CanDisable: true},
		},
		{
			name: "a stop row answers no reasoning", provider: "xai", model: "grok-2-image-1212",
			want: thinkingAnswer{Reasoning: false, CanDisable: true},
		},
		{
			name: "an unknown id answers the floor", provider: "openai", model: "totally-unknown-9000",
			want: thinkingAnswer{Reasoning: false, CanDisable: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Capabilities(tc.provider, tc.model)
			answer := thinkingAnswer{Reasoning: got.Reasoning, Format: got.ThinkingFormat, CanDisable: got.CanDisable}
			if answer != tc.want {
				t.Fatalf("Capabilities(%q, %q) = %+v, want %+v", tc.provider, tc.model, answer, tc.want)
			}
		})
	}
}

// TestThinkingLevels_FormatSets pins one model per format the kept set reaches,
// including the two the reference answers with a "thinking" binary instead of a
// level, and a model whose format the reference left unset.
func TestThinkingLevels_FormatSets(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		model    string
		want     []string
	}{
		{
			name: "the openai set has no max", provider: "openai", model: "gpt-5.5",
			want: []string{"none", "minimal", "low", "medium", "high", "xhigh"},
		},
		{
			name: "claude-adaptive stops at max", provider: "claude", model: "claude-opus-4-7",
			want: []string{"none", "low", "medium", "high", "max"},
		},
		{
			name: "claude-budget adds xhigh", provider: "claude", model: "claude-3-5-sonnet-20241022",
			want: []string{"none", "low", "medium", "high", "xhigh", "max"},
		},
		{
			name: "zai is a binary switch", provider: "glm", model: "glm-4.6",
			want: []string{"none", "thinking"},
		},
		{
			name: "a reasoner that cannot disable loses none", provider: "deepseek", model: "deepseek-reasoner",
			want: []string{"high", "max"},
		},
		{
			name: "a chat model the reference denies answers no levels", provider: "deepseek", model: "deepseek-chat",
			want: nil,
		},
		{
			name: "a model that cannot disable loses none", provider: "kimi", model: "kimi-k3",
			want: []string{"low", "medium", "high", "max"},
		},
		{
			name: "the openai floor answers no levels", provider: "openai", model: "totally-unknown-9000",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ThinkingLevels(tc.provider, tc.model)
			if len(got) != len(tc.want) {
				t.Fatalf("ThinkingLevels(%q, %q) = %v, want %v", tc.provider, tc.model, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ThinkingLevels(%q, %q) = %v, want %v", tc.provider, tc.model, got, tc.want)
				}
			}
		})
	}
}
