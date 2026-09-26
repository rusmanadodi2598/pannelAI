// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/intent_test.go
// @for       The client-intent extractor: every shape the three client wires
//
//	can carry, and the priority between them.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.15 makes the client's own intent outrank the stored
//
//	provider mode. An extractor that misses a shape is a client whose
//	explicit choice is silently overridden, so every shape the port
//	accepts is pinned here, including the ones the gateway's own DTOs do
//	not model but a raw body can still carry.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "testing"

// TestExtract_Shapes pins every intent shape and its priority.
func TestExtract_Shapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want *Config
	}{
		{"no body", ``, nil},
		{"an empty object", `{}`, nil},
		{"a body that is not an object", `[]`, nil},
		{"no reasoning field", `{"model":"gpt-5.5","messages":[]}`, nil},
		{"an OpenAI effort", `{"reasoning_effort":"high"}`, &Config{Mode: "level", Level: "high"}},
		{"an upper-cased effort", `{"reasoning_effort":"HIGH"}`, &Config{Mode: "level", Level: "high"}},
		{"an OpenAI none", `{"reasoning_effort":"none"}`, &Config{Mode: "none"}},
		{"an OpenAI off", `{"reasoning_effort":"off"}`, &Config{Mode: "none"}},
		{"an OpenAI auto", `{"reasoning_effort":"auto"}`, &Config{Mode: "auto"}},
		{"a Responses effort", `{"reasoning":{"effort":"medium","summary":"auto"}}`, &Config{Mode: "level", Level: "medium"}},
		{"a Claude disabled block", `{"thinking":{"type":"disabled"}}`, &Config{Mode: "none"}},
		{"a Claude budget block", `{"thinking":{"type":"enabled","budget_tokens":4096}}`, &Config{Mode: "budget", Budget: 4096}},
		{"a Claude bare enable", `{"thinking":{"type":"enabled"}}`, &Config{Mode: "auto"}},
		{"a Claude adaptive block", `{"thinking":{"type":"adaptive"}}`, &Config{Mode: "auto"}},
		{"a Claude effort", `{"output_config":{"effort":"xhigh"}}`, &Config{Mode: "level", Level: "xhigh"}},
		{"the effort outranks the thinking block", `{"output_config":{"effort":"low"},"thinking":{"type":"disabled"}}`, &Config{Mode: "level", Level: "low"}},
		{"the effort outranks a thinking object", `{"reasoning_effort":"high","thinking":{"type":"enabled"}}`, &Config{Mode: "level", Level: "high"}},
		{"a Gemini level", `{"thinkingConfig":{"thinkingLevel":"HIGH"}}`, &Config{Mode: "level", Level: "high"}},
		{"a Gemini budget of zero", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":0}}}`, &Config{Mode: "none"}},
		{"a Gemini budget of minus one", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":-1}}}`, &Config{Mode: "auto"}},
		{"a Gemini budget", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":5000}}}`, &Config{Mode: "budget", Budget: 5000}},
		{"a Gemini request envelope", `{"request":{"generationConfig":{"thinkingConfig":{"thinkingLevel":"low"}}}}`, &Config{Mode: "level", Level: "low"}},
		{"a Qwen disable", `{"enable_thinking":false}`, &Config{Mode: "none"}},
		{"a Qwen enable", `{"enable_thinking":true}`, &Config{Mode: "auto"}},
		{"a Qwen budget", `{"enable_thinking":true,"thinking_budget":2048}`, &Config{Mode: "budget", Budget: 2048}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Extract([]byte(tc.body))
			if !sameConfig(got, tc.want) {
				t.Fatalf("Extract(%s) = %+v, want %+v", tc.body, got, tc.want)
			}
		})
	}
}

// sameConfig compares two optional configs by value.
func sameConfig(got, want *Config) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}
