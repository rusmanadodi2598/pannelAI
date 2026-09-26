// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/apply_test.go
// @for       The strip-then-write rule, the display read, the budget
//
//	conversion, and the doubles every format case is driven through.
//
// @uses      encoding/json, reflect, testing.
// @reason    SPEC-API-001 §7.15 ports thinkingUnified.js, whose two halves are
//
//	"remove whatever the client sent" and "write what this upstream reads".
//	The strip half is proven here in one place because it is one rule
//	applied to every shape; the per-format writes live in
//	apply_wire_test.go and apply_vendor_test.go (AGENTS.md §1.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import (
	"encoding/json"
	"reflect"
	"testing"
)

// applyCase is one format case: the body it starts from, the format and config
// it applies, and the body it must end as.
type applyCase struct {
	name    string
	body    string
	format  string
	cfg     Config
	caps    modelCaps
	display string
	want    string
}

// runApplyCases drives one table, so a format family split across files keeps
// the same execution and the same failure message.
func runApplyCases(t *testing.T, cases []applyCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyToJSON(t, tc.body, tc.format, tc.cfg, tc.caps, tc.display)
			assertJSON(t, got, tc.want)
		})
	}
}

// TestStripAll_RemovesEveryShape pins the strip half: a re-applied config must
// not leave a stale member from another format beside the new one, including
// the nested generationConfig and the commandcode params envelope.
func TestStripAll_RemovesEveryShape(t *testing.T) {
	body := `{"thinking":{"type":"enabled"},"reasoning_effort":"high","reasoning":{"effort":"low"},` +
		`"thinkingConfig":{"thinkingLevel":"high"},"enable_thinking":true,"thinking_budget":4096,` +
		`"output_config":{"effort":"high"},"generationConfig":{"thinkingConfig":{"thinkingLevel":"low"},"temperature":0.1},` +
		`"request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":4096},"maxOutputTokens":100}},` +
		`"params":{"reasoning_effort":"high","thinking":{"type":"enabled"},"temperature":0.2}}`
	decoded, ok := decodeObject([]byte(body))
	if !ok {
		t.Fatal("the fixture did not decode")
	}
	stripAll(decoded)
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	assertJSON(t, decodeAny(t, encoded),
		`{"generationConfig":{"temperature":0.1},"request":{"generationConfig":{"maxOutputTokens":100}},"params":{"temperature":0.2}}`)
}

// TestThinkingDisplay_ReadsTheFlag pins the display read that keeps a client's
// summarized-thinking choice across the strip.
func TestThinkingDisplay_ReadsTheFlag(t *testing.T) {
	decoded, _ := decodeObject([]byte(`{"thinking":{"type":"enabled","display":"omitted"}}`))
	if got := thinkingDisplay(decoded); got != "omitted" {
		t.Fatalf("thinkingDisplay() = %q, want omitted", got)
	}
	decoded, _ = decodeObject([]byte(`{"thinking":{"type":"enabled"}}`))
	if got := thinkingDisplay(decoded); got != "" {
		t.Fatalf("thinkingDisplay() = %q, want empty", got)
	}
}

// TestToBudget_ClampsAndDistinguishes pins the budget conversion, where an
// unresolvable config and a zero budget are different answers.
func TestToBudget_ClampsAndDistinguishes(t *testing.T) {
	cases := []struct {
		name  string
		cfg   Config
		caps  modelCaps
		want  int
		found bool
	}{
		{"a named budget", Config{Mode: "budget", Budget: 4096}, modelCaps{}, 4096, true},
		{"a level", Config{Mode: "level", Level: "high"}, modelCaps{}, 24576, true},
		{"an unknown level", Config{Mode: "level", Level: "bogus"}, modelCaps{}, 0, false},
		{"auto is the upstream default", Config{Mode: "auto"}, modelCaps{}, -1, true},
		{"none names nothing", Config{Mode: "none"}, modelCaps{}, 0, false},
		{"a zero budget stays zero", Config{Mode: "budget", Budget: 0}, modelCaps{}, 0, true},
		{
			name: "a range raises the floor", cfg: Config{Mode: "budget", Budget: 100},
			caps: modelCaps{hasRange: true, rangeMin: 1024, rangeMax: 24576}, want: 1024, found: true,
		},
		{
			name: "a range lowers the ceiling", cfg: Config{Mode: "budget", Budget: 99999},
			caps: modelCaps{hasRange: true, rangeMin: 1024, rangeMax: 24576}, want: 24576, found: true,
		},
		{
			name: "auto is never clamped", cfg: Config{Mode: "auto"},
			caps: modelCaps{hasRange: true, rangeMin: 1024, rangeMax: 24576}, want: -1, found: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := toBudget(tc.cfg, tc.caps)
			if got != tc.want || found != tc.found {
				t.Fatalf("toBudget(%+v) = (%d, %v), want (%d, %v)", tc.cfg, got, found, tc.want, tc.found)
			}
		})
	}
}

// applyToJSON applies one config to a JSON body and returns the decoded result,
// so a case states the whole body rather than one field.
func applyToJSON(t *testing.T, body, format string, cfg Config, caps modelCaps, display string) map[string]any {
	t.Helper()
	decoded, ok := decodeObject([]byte(body))
	if !ok {
		t.Fatalf("the fixture %s did not decode", body)
	}
	applyFormat(decoded, format, cfg, caps, display)
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return decodeAny(t, encoded)
}

// decodeAny decodes a JSON object into the generic shape a case compares.
func decodeAny(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", raw, err)
	}
	return decoded
}

// assertJSON compares a decoded body against an expected JSON literal, so a
// case states the member order it does not care about.
func assertJSON(t *testing.T, got map[string]any, want string) {
	t.Helper()
	var expected map[string]any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatalf("the expectation %s is not JSON: %v", want, err)
	}
	if !reflect.DeepEqual(got, expected) {
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("the body under test could not be re-encoded: %v", err)
		}
		t.Fatalf("body = %s, want %s", encoded, want)
	}
}
