// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/pricing_test.go
// @for       The fallback chain and the token-to-cost formula, over a table.
// @uses      testing, internal/registry.
// @reason    The rate tables and the cost formula are a port of the reference's
//
//	pricing.js. The pair-by-pair comparison against the reference's own
//	answers lives in pricing_corpus_test.go; what is pinned here are the
//	order and the arithmetic edges a corpus cannot isolate — the
//	provider override before the canonical model, the inclusive-prompt
//	cache split, the clamp when the cache subsets exceed the prompt, the
//	reasoning premium, and the zero answer for an unpriced model.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package registry

import (
	"testing"
)

// TestPricing_FallbackChainOrder pins the resolution order: the
// provider-specific override first, then the canonical model with its vendor
// prefix stripped, then the ordered pattern table, then nothing. The
// free-suffixed case is deliberate parity: neither the reference nor this port
// treats ":free" as a special case, so the model prices through its family
// pattern.
func TestPricing_FallbackChainOrder(t *testing.T) {
	cases := []struct {
		name       string
		provider   string
		model      string
		wantPriced bool
		wantInput  string
	}{
		{"provider override wins over canonical", "tokenrouter", "deepseek/deepseek-v3.2", true, "0.26"},
		{"canonical model beats the pattern table", "anything", "deepseek-chat", true, "0.14"},
		{"vendor-prefixed free-suffixed model resolves through its pattern", "th-1", "deepseek-v4.1-flash:free", true, "0.14"},
		{"case-insensitive pattern match", "anything", "DeepSeek-Chat", true, "0.14"},
		{"unpriced model answers not priced", "anything", "totally-unknown-model", false, ""},
		{"empty model answers not priced", "anything", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rates, priced := ResolvePricing(tc.provider, tc.model)
			if priced != tc.wantPriced {
				t.Fatalf("priced = %v, want %v", priced, tc.wantPriced)
			}
			if priced && rates.Input != tc.wantInput {
				t.Fatalf("input rate = %q, want %q", rates.Input, tc.wantInput)
			}
		})
	}
}

// TestPricing_CostFormula pins the token-to-cost conventions over a table: the
// inclusive-prompt split (cached and cache-creation are subsets, subtracted
// before the full input rate applies), the clamp when the subsets exceed the
// prompt, the reasoning premium on its own rate, the cached-rate fallback to
// the input rate, the exact 8-decimal rendering, and the zero answers for an
// empty report.
func TestPricing_CostFormula(t *testing.T) {
	rates := Rate{Input: "3", Output: "15", Cached: "0.3", CacheCreation: "3.75", Reasoning: "15"}

	cases := []struct {
		name   string
		tokens TokenCounts
		rate   Rate
		want   string
	}{
		{"prompt-inclusive cache split, no double-count", TokenCounts{Prompt: 330, Completion: 50, Cached: 200, CacheCreation: 30}, rates, "0.00122250"},
		{"subsets exceeding the prompt clamp to zero", TokenCounts{Prompt: 100, Cached: 80, CacheCreation: 40}, rates, "0.00017400"},
		{"plain pricing when no cache is present", TokenCounts{Prompt: 100, Completion: 50}, rates, "0.00105000"},
		{"reasoning tokens add their premium rate on top of output", TokenCounts{Completion: 100, Reasoning: 40}, rates, "0.00210000"},
		{"cached rate falls back to the input rate when absent", TokenCounts{Prompt: 100, Cached: 40}, Rate{Input: "3", Output: "15", Reasoning: "15"}, "0.00030000"},
		{"a billion input tokens at $5/M is $5000, exactly", TokenCounts{Prompt: 1000000000}, Rate{Input: "5", Output: "25"}, "5000.00000000"},
		{"an empty report answers the column scale", TokenCounts{}, rates, "0.00000000"},
		{"six-decimal rates render at the column scale", TokenCounts{Prompt: 1500000, Completion: 500000}, Rate{Input: "0.000003", Output: "0.000007"}, "0.00000800"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CalculateCost(tc.tokens, tc.rate); got != tc.want {
				t.Fatalf("cost = %q, want %q", got, tc.want)
			}
		})
	}
}
