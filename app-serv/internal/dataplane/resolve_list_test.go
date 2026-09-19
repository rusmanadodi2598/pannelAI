// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/resolve_list_test.go
// @for       Table-driven tests for the models list and the translator gate.
// @uses      context, strings, testing, internal/domain, internal/registry
// @reason    SPEC-API-001 §7.15 makes the models list a catalog of what is routable and §8 makes an
//
//	untranslated format a named refusal, so both are one decision read two ways: a model is
//	listed when a request for it could be served. Both are pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestResolver_ModelList pins the models list: routable models only, disabled
// pairs hidden, combos owned by "combo", and a stable order.
func TestResolver_ModelList(t *testing.T) {
	lookup := fakeLookup{
		combos:  map[string]domain.Combo{"my-combo": comboRow("my-combo", "provider-a/x")},
		aliases: map[string]string{},
		disabled: func() []domain.ModelRef {
			ref, err := domain.NewModelRef("declared", "known-model")
			if err != nil {
				t.Fatalf("building disabled ref: %v", err)
			}
			return []domain.ModelRef{ref}
		}(),
	}
	resolver, err := NewResolver(testIndex(t), lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}

	list, err := resolver.ModelList(context.Background())
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}
	if list.Object != "list" {
		t.Fatalf("Object = %q, want list", list.Object)
	}

	for _, entry := range list.Data {
		if entry.Object != "model" {
			t.Fatalf("entry %q object = %q, want model", entry.ID, entry.Object)
		}
	}
	// The list is exactly the chat-capable declared models of translatable
	// providers, plus every combo. A passthrough provider enumerates nothing, an
	// image model is not chat, and a disabled pair is hidden.
	want := []string{"declared/claude-native", "declared/exposed-model", "gated/secret-model", "my-combo"}
	got := make([]string, 0, len(list.Data))
	for _, entry := range list.Data {
		got = append(got, entry.ID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("models list = %v, want %v", got, want)
	}
	for i := 1; i < len(list.Data); i++ {
		if list.Data[i-1].ID > list.Data[i].ID {
			t.Fatalf("models list is not sorted: %q before %q", list.Data[i-1].ID, list.Data[i].ID)
		}
	}
}

// TestTargetFormat pins the translator check: only a format with a translator
// decodes as routable, so a Gemini provider is refused by name rather than sent
// as a 502. The Responses format gained its translator in P3 (SPEC-API-001 §10),
// so it now maps to its own target.
func TestTargetFormat(t *testing.T) {
	cases := []struct {
		format string
		want   string
	}{
		{format: registry.DefaultFormat, want: TargetOpenAI},
		{format: "claude", want: TargetClaude},
		{format: "gemini", want: ""},
		{format: "gemini-cli", want: ""},
		{format: registry.FormatOpenAIResponses, want: TargetResponses},
		{format: "kiro", want: ""},
		{format: "", want: ""},
	}
	for _, tc := range cases {
		t.Run("format="+tc.format, func(t *testing.T) {
			if got := targetFormat(tc.format); got != tc.want {
				t.Fatalf("targetFormat(%q) = %q, want %q", tc.format, got, tc.want)
			}
		})
	}
}
