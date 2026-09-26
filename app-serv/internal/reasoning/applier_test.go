// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/applier_test.go
// @for       The request-path seam's doubles and the cases that must leave a
//
//	body untouched, plus the shape rules its output obeys.
//
// @uses      context, encoding/json, errors, testing, internal/domain,
//
//	internal/registry.
//
// @reason    SPEC-API-001 §7.14 stores one mode per provider and §7.15 carries
//
//	the suffix; AGENTS.md §2.1 requires the cases proven beside the seam,
//	because every wrong answer here is silent: a body that gained a
//	field the client did not ask for, or lost one it did. The precedence
//	table lives in applier_precedence_test.go (AGENTS.md §1.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package reasoning

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stubSettings answers the typed document the applier reads, so no test needs a
// database.
type stubSettings struct {
	settings domain.Settings
	err      error
}

func (s stubSettings) Settings(context.Context) (domain.Settings, error) {
	return s.settings, s.err
}

// settingsWithThinking builds a document carrying one provider mode.
func settingsWithThinking(provider string, mode domain.ThinkingMode) domain.Settings {
	settings := domain.DefaultSettings()
	settings.Reasoning.ProviderThinking = map[string]domain.ProviderThinking{
		provider: {Mode: mode},
	}
	return settings
}

// TestApplier_LeavesTheBodyAlone pins the cases where nothing may change: no
// config, a model the resolver says does not reason, an unreadable body, and a
// settings read that failed. An optional control must never fail a call.
func TestApplier_LeavesTheBodyAlone(t *testing.T) {
	applier, err := NewApplier(stubSettings{settings: settingsWithThinking("openai", "high")})
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}
	cases := []struct {
		name string
		body string
		call Call
	}{
		{"no config for the provider", `{"model":"m"}`,
			Call{Wire: "claude", ProviderID: "anthropic", ModelID: "claude-opus-4-8"}},
		{"a model the resolver does not know", `{"model":"m"}`,
			Call{Wire: "openai", ProviderID: "my-node", ModelID: "mystery-1"}},
		{"a model that does not reason", `{"model":"m","reasoning_effort":"high"}`,
			Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-image-1"}},
		{"a body that is not an object", `[]`,
			Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5"}},
		{"an empty body", ``,
			Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := applier.Apply(context.Background(), []byte(tc.body), tc.call); string(got) != tc.body {
				t.Fatalf("Apply() = %s, want the body unchanged (%s)", got, tc.body)
			}
		})
	}

	failing, err := NewApplier(stubSettings{err: errors.New("settings store unavailable")})
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}
	body := `{"model":"m"}`
	call := Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5"}
	if got := failing.Apply(context.Background(), []byte(body), call); string(got) != body {
		t.Fatalf("Apply() = %s, want the body unchanged on a read failure", got)
	}
}

// TestApplier_AStrippedClientIntentIsReapplied pins the other half of "leave the
// client alone": when the client did carry an intent, the body is rewritten in
// the target's shape rather than left in the client's, which is what carries it
// across a translation.
func TestApplier_AStrippedClientIntentIsReapplied(t *testing.T) {
	applier, err := NewApplier(stubSettings{settings: domain.DefaultSettings()})
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}
	body := `{"model":"m","reasoning_effort":"low","thinking":{"type":"enabled"}}`
	call := Call{Wire: "claude", ProviderID: "anthropic", ModelID: "claude-opus-4-8",
		ClientRaw: []byte(`{"reasoning_effort":"low"}`)}
	got := applier.Apply(context.Background(), []byte(body), call)
	assertJSON(t, decodeAny(t, got), `{"model":"m","thinking":{"type":"adaptive"},"output_config":{"effort":"low"}}`)
}

// TestResolveFormat pins the wire/format decision, including the guard that
// keeps an OpenAI-wire upstream from receiving a Claude or Gemini shape.
func TestResolveFormat(t *testing.T) {
	cases := []struct {
		name   string
		wire   string
		format string
		want   string
	}{
		{"a declared format is used as it stands", "openai", "zai", "zai"},
		{"the commandcode wire keeps its format", "openai", "commandcode", "commandcode"},
		{"a Claude format on the Claude wire is used", "claude", "claude-adaptive", "claude-adaptive"},
		{"a Claude format on the OpenAI wire falls back", "openai", "claude-budget", "openai"},
		{"a Gemini format on the OpenAI wire falls back", "openai", "gemini-level", "openai"},
		{"a Gemini format on the Responses wire falls back", "openai-responses", "gemini-budget", "openai"},
		{"no declared format on the OpenAI wire", "openai", "", "openai"},
		{"no declared format on the Responses wire", "openai-responses", "", "openai"},
		{"no declared format on the Claude wire", "claude", "", "claude-budget"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caps := registry.CapabilitySet{ThinkingFormat: tc.format}
			if got := resolveFormat(tc.wire, caps); got != tc.want {
				t.Fatalf("resolveFormat(%q, format %q) = %q, want %q", tc.wire, tc.format, got, tc.want)
			}
		})
	}
}

// TestConfigFromMode pins the stored mode's mapping, including the two states
// that are not levels.
func TestConfigFromMode(t *testing.T) {
	cases := []struct {
		mode domain.ThinkingMode
		want Config
	}{
		{domain.ThinkingOn, Config{Mode: "budget", Budget: OnBudget}},
		{domain.ThinkingOff, Config{Mode: "none"}},
		{domain.ThinkingMode("none"), Config{Mode: "none"}},
		{domain.ThinkingMode("high"), Config{Mode: "level", Level: "high"}},
		{domain.ThinkingMode("thinking"), Config{Mode: "level", Level: "thinking"}},
	}
	for _, tc := range cases {
		if got := configFromMode(tc.mode); *got != tc.want {
			t.Fatalf("configFromMode(%q) = %+v, want %+v", tc.mode, *got, tc.want)
		}
	}
}

// TestNewApplier_RequiresSettings pins the constructor's one rule.
func TestNewApplier_RequiresSettings(t *testing.T) {
	if _, err := NewApplier(nil); err == nil {
		t.Fatal("NewApplier(nil) error = nil, want the missing port refused")
	}
}

// TestApply_EncodesOneObject pins that the applied body stays a single JSON
// object, so a caller can send it without re-checking.
func TestApply_EncodesOneObject(t *testing.T) {
	applier, err := NewApplier(stubSettings{settings: settingsWithThinking("openai", "high")})
	if err != nil {
		t.Fatalf("NewApplier() error = %v", err)
	}
	got := applier.Apply(context.Background(), []byte(`{"model":"m"}`),
		Call{Wire: "openai", ProviderID: "openai", ModelID: "gpt-5.5"})
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("the applied body is not one object: %v", err)
	}
}
