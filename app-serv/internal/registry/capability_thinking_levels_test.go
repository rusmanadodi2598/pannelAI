// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_thinking_levels_test.go
// @for       The level vocabulary: the rows that override a format's set, and
//
//	the two corpus-level checks every level must pass.
//
// @uses      testing, internal/domain.
// @reason    SPEC-API-001 §7.15 offers one picker per provider built from the
//
//	union of its models' levels, so a level the resolver offers must be
//	one the reasoning group stores: a mismatch is a picker entry whose
//	PATCH the gateway rejects. The group is contract and the tables are
//	catalog, so the two packages cannot import each other and the pin
//	lives here, on the tokensaver tests' precedent. The layer boundaries
//	themselves are proven in capability_thinking_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package registry

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestThinkingLevels_PatternOverrides pins the model-name rows the reference
// carries: a provider-scoped row that only matches its provider, and a generic
// row that replaces the format set.
func TestThinkingLevels_PatternOverrides(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		model    string
		want     []string
	}{
		{
			name: "codebuddy-cn's glm-5.3 offers three levels", provider: "codebuddy-cn", model: "glm-5.3",
			want: []string{"low", "high", "max"},
		},
		{
			name: "the same model id under another provider keeps its format set", provider: "glm", model: "glm-5.3",
			want: []string{"none", "thinking"},
		},
		{
			name: "the codex row is generic, so it also shapes a github model", provider: "github", model: "gpt-5-codex",
			want: []string{"low", "medium", "high", "xhigh"},
		},
		{
			name: "the deepseek-v4 row replaces the deepseek set", provider: "deepseek", model: "deepseek-v4.1-flash",
			want: []string{"none", "low", "medium", "high", "xhigh", "max"},
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

// TestThinkingLevels_EveryReasoningPairHasLevels is the corpus-level sanity
// check: a model the resolver calls reasoning must answer a level set, and a
// model it does not must answer none. It catches a format name the level table
// does not know, which would otherwise surface as an empty picker.
func TestThinkingLevels_EveryReasoningPairHasLevels(t *testing.T) {
	corpus := loadCapabilityCorpus(t)
	reasoning, silent := 0, 0
	for _, entry := range corpus.Entries {
		levels := ThinkingLevels(entry.Provider, entry.Model)
		if entry.Reasoning {
			reasoning++
			if len(levels) == 0 {
				t.Fatalf("ThinkingLevels(%q, %q) = none, but the reference says the model reasons (format %q)",
					entry.Provider, entry.Model, entry.ThinkingFormat)
			}
			continue
		}
		silent++
		if len(levels) != 0 {
			t.Fatalf("ThinkingLevels(%q, %q) = %v, but the reference says the model does not reason",
				entry.Provider, entry.Model, levels)
		}
	}
	t.Logf("levels answered for %d reasoning pairs; %d non-reasoning pairs answered none", reasoning, silent)
}

// TestThinkingLevels_EveryLevelIsStorable pins the level vocabulary against the
// reasoning group's stored modes: a level the resolver offers but the group
// refuses would be a picker entry whose PATCH the gateway rejects. The group is
// contract and the tables are catalog, so the two packages cannot import each
// other and the pin lives here, on the tokensaver tests' precedent.
func TestThinkingLevels_EveryLevelIsStorable(t *testing.T) {
	check := func(source, level string) {
		t.Helper()
		if !domain.ValidThinkingMode(domain.ThinkingMode(level)) {
			t.Fatalf("%s offers level %q, which reasoning.provider_thinking refuses", source, level)
		}
	}
	for format, levels := range thinkingFormatLevels {
		for _, level := range levels {
			check("format "+format, level)
		}
	}
	for _, rule := range thinkingLevelRules {
		for _, level := range rule.levels {
			check("rule "+rule.pattern, level)
		}
	}
}
