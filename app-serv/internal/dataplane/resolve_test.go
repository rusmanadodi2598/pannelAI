// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/resolve_test.go
// @for       Table-driven tests for the model-string resolution order.
// @uses      context, testing, internal/domain
// @reason    SPEC-API-001 §7.15 fixes the order a model string resolves in: combo name, then alias,
//
//	then provider/model. A request that resolves wrongly is served by the wrong account, so
//	every step and every refusal code is pinned here (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestResolver_Order pins the documented resolution order (SPEC-API-001 §7.15):
// combo name, then alias, then provider/model, then MODEL_NOT_FOUND — including
// each failure and the provider-not-routable case §8 adds.
func TestResolver_Order(t *testing.T) {
	lookup := fakeLookup{
		combos: map[string]domain.Combo{
			"my-combo":   comboRow("my-combo", "provider-a/fast", "claude-only/slow"),
			"nested":     comboRow("nested", "my-combo"),
			"empty":      comboRow("empty"),
			"claude-com": comboRow("claude-com", "claude-only/x"),
		},
		aliases: map[string]string{
			"fast":     "provider-a/gpt-fast",
			"shortcut": "my-combo",
			"to-bad":   "connector-only/model",
			"chain":    "fast",
		},
	}

	cases := []struct {
		name         string
		model        string
		wantProvider string
		wantModel    string
		wantUpstream string
		wantTarget   string
		wantCombo    string
		wantErrCode  string
	}{
		{
			name:  "a combo name resolves to its first member and keeps the combo",
			model: "my-combo", wantProvider: "provider-a", wantModel: "fast",
			wantUpstream: "fast", wantTarget: TargetOpenAI, wantCombo: "my-combo",
		},
		{
			name:  "a combo whose first member is a claude provider keeps that target",
			model: "claude-com", wantProvider: "claude-only", wantModel: "x",
			wantUpstream: "x", wantTarget: TargetClaude, wantCombo: "claude-com",
		},
		{
			name:  "an alias resolves to its provider and model",
			model: "fast", wantProvider: "provider-a", wantModel: "gpt-fast", wantUpstream: "gpt-fast",
			wantTarget: TargetOpenAI,
		},
		{
			name:  "an alias pointing at a combo resolves through it",
			model: "shortcut", wantProvider: "provider-a", wantModel: "fast",
			wantUpstream: "fast", wantTarget: TargetOpenAI, wantCombo: "my-combo",
		},
		{
			name:  "a provider alias resolves as the provider",
			model: "pa/thing", wantProvider: "provider-a", wantModel: "thing",
			wantUpstream: "thing", wantTarget: TargetOpenAI,
		},
		{
			name:  "a declared model uses its upstream override",
			model: "declared/exposed-model", wantProvider: "declared", wantModel: "exposed-model",
			wantUpstream: "real-model", wantTarget: TargetOpenAI,
		},
		{
			name:  "a declared model may override the target format",
			model: "declared/claude-native", wantProvider: "declared", wantModel: "claude-native",
			wantUpstream: "claude-native", wantTarget: TargetClaude,
		},
		{
			name:  "a passthrough provider accepts an undeclared model",
			model: "provider-a/anything", wantProvider: "provider-a", wantModel: "anything",
			wantUpstream: "anything", wantTarget: TargetOpenAI,
		},
		{
			name:  "a model a non-passthrough provider does not declare is MODEL_NOT_FOUND",
			model: "declared/missing", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "an unknown provider is MODEL_NOT_FOUND",
			model: "nope/model", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "an unknown bare name is MODEL_NOT_FOUND",
			model: "nothing-like-this", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "a model string with no provider segment is MODEL_NOT_FOUND",
			model: "justaname/", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "an empty model string is MODEL_NOT_FOUND",
			model: "", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "a provider whose protocol has no translator is PROVIDER_NOT_ROUTABLE",
			model: "connector-only/model", wantErrCode: CodeProviderNotRoutable,
		},
		{
			name:  "an alias pointing at an unroutable provider is PROVIDER_NOT_ROUTABLE",
			model: "to-bad", wantErrCode: CodeProviderNotRoutable,
		},
		{
			name:  "an empty combo falls through to the alias path",
			model: "empty", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "an alias chain terminates at the model",
			model: "chain", wantProvider: "provider-a", wantModel: "gpt-fast",
			wantUpstream: "gpt-fast", wantTarget: TargetOpenAI,
		},
		{
			name:  "a nested combo member resolves to the inner combo's member",
			model: "nested", wantProvider: "provider-a", wantModel: "fast",
			wantUpstream: "fast", wantTarget: TargetOpenAI, wantCombo: "nested",
		},
		{
			name:  "a hidden provider's model is still resolvable by id",
			model: "gated/secret-model", wantProvider: "gated", wantModel: "secret-model",
			wantUpstream: "secret-model", wantTarget: TargetClaude,
		},
	}

	resolver, err := NewResolver(testIndex(t), lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolver.Resolve(context.Background(), tc.model)
			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatalf("err = nil, want %s (got %+v)", tc.wantErrCode, got)
				}
				if code := AsError(err).Code; code != tc.wantErrCode {
					t.Fatalf("code = %q, want %q (message: %s)", code, tc.wantErrCode, AsError(err).Message)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want none", err)
			}
			if got.Provider.ID != tc.wantProvider {
				t.Fatalf("provider = %q, want %q", got.Provider.ID, tc.wantProvider)
			}
			if got.ModelID != tc.wantModel {
				t.Fatalf("model = %q, want %q", got.ModelID, tc.wantModel)
			}
			if got.UpstreamID != tc.wantUpstream {
				t.Fatalf("upstream id = %q, want %q", got.UpstreamID, tc.wantUpstream)
			}
			if got.Target != tc.wantTarget {
				t.Fatalf("target = %q, want %q", got.Target, tc.wantTarget)
			}
			if got.Combo.Name() != tc.wantCombo {
				t.Fatalf("combo = %q, want %q", got.Combo.Name(), tc.wantCombo)
			}
		})
	}
}
