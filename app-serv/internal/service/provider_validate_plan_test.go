// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_validate_plan_test.go
// @for       The per-format validation plan, measured over the REAL registry.
//
// @uses      internal/registry, testing.
// @reason    Draft 017 §4.2's second consequence is that 76 providers answered
//
//	"this provider does not declare a validation endpoint" while the
//	reference could check most of them. The plan is a small pure function,
//	so it is asserted twice: as a table of shapes, and against every entry
//	the embedded document declares — the second is the one that would have
//	caught the original gap, because it names the providers left uncovered
//	instead of trusting a hand-written sample.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestPlanFor_Shapes covers each rule and the boundary that yields no plan.
func TestPlanFor_Shapes(t *testing.T) {
	cases := []struct {
		name          string
		entry         registry.Provider
		wantURL       string
		wantMethod    string
		wantAnthropic bool
		wantNeedsBody bool
	}{
		{
			name: "a declared validate URL wins",
			entry: registry.Provider{Transport: registry.Transport{
				BaseURL: "https://x.test/v1/chat/completions", ValidateURL: "https://x.test/v1/models"}},
			wantURL: "https://x.test/v1/models", wantMethod: "GET",
		},
		{
			name:    "a chat completions base derives the models path",
			entry:   registry.Provider{Transport: registry.Transport{BaseURL: "https://x.test/v1/chat/completions", Format: "openai"}},
			wantURL: "https://x.test/v1/models", wantMethod: "GET",
		},
		{
			name:    "a chatbot base derives the models path",
			entry:   registry.Provider{Transport: registry.Transport{BaseURL: "https://x.test/chatbot", Format: "openai"}},
			wantURL: "https://x.test/models", wantMethod: "GET",
		},
		{
			name:          "an anthropic messages base is probed by POST",
			entry:         registry.Provider{Transport: registry.Transport{BaseURL: "https://api.anthropic.com/v1/messages", Format: "claude"}},
			wantURL:       "https://api.anthropic.com/v1/messages",
			wantMethod:    "POST",
			wantAnthropic: true,
			wantNeedsBody: true,
		},
		{
			name:          "a claude-format chat path keeps the Anthropic rule",
			entry:         registry.Provider{Transport: registry.Transport{BaseURL: "https://x.test/v1/chat/completions", Format: "claude"}},
			wantURL:       "https://x.test/v1/models",
			wantMethod:    "GET",
			wantAnthropic: true,
		},
		{
			name:    "a gemini base is already its own models path",
			entry:   registry.Provider{Transport: registry.Transport{BaseURL: "https://generativelanguage.googleapis.com/v1beta/models", Format: "gemini"}},
			wantURL: "https://generativelanguage.googleapis.com/v1beta/models", wantMethod: "GET",
		},
		{
			name:  "a base naming neither yields no plan",
			entry: registry.Provider{Transport: registry.Transport{BaseURL: "https://x.test/opaque", Format: "openai"}},
		},
		{
			name:  "an empty base yields no plan",
			entry: registry.Provider{Transport: registry.Transport{Format: "openai"}},
		},
		{
			name:    "a declared URL is trimmed",
			entry:   registry.Provider{Transport: registry.Transport{ValidateURL: "  https://x.test/v1/models  "}},
			wantURL: "https://x.test/v1/models", wantMethod: "GET",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := PlanFor(tc.entry)
			if plan.URL != tc.wantURL {
				t.Fatalf("URL = %q, want %q", plan.URL, tc.wantURL)
			}
			if plan.Method != tc.wantMethod {
				t.Fatalf("Method = %q, want %q", plan.Method, tc.wantMethod)
			}
			if plan.AnthropicRule != tc.wantAnthropic {
				t.Fatalf("AnthropicRule = %v, want %v", plan.AnthropicRule, tc.wantAnthropic)
			}
			if plan.NeedsModel != tc.wantNeedsBody {
				t.Fatalf("NeedsModel = %v, want %v", plan.NeedsModel, tc.wantNeedsBody)
			}
		})
	}
}

// TestPlanFor_CoversMostOfTheRealRegistry is the measurement draft §4.2 asks for,
// taken against the embedded document rather than a sample.
//
// It does not demand 100%: a provider whose base URL is neither a chat path nor a
// models path genuinely has no checkable surface without a connector, and
// reporting that is honest. What it pins is that the count is a known figure and
// that the Anthropic-wire family — the one draft §4.6 names as the common case —
// is covered.
func TestPlanFor_CoversMostOfTheRealRegistry(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	covered, uncovered := 0, 0
	claudeCovered, claudeTotal := 0, 0
	for _, entry := range index.All() {
		if entry.Transport.BaseURL == "" && entry.Transport.ValidateURL == "" {
			continue // a media-only provider carries no chat transport
		}
		plan := PlanFor(entry)
		if entry.Transport.Format == "claude" {
			claudeTotal++
			if plan.URL != "" {
				claudeCovered++
			}
		}
		if plan.URL == "" {
			uncovered++
			continue
		}
		covered++
		if plan.Method == "" {
			t.Fatalf("%s has a URL but no method; a plan must say how to ask", entry.ID)
		}
	}
	if covered < 37 {
		t.Fatalf("only %d entries have a checkable surface; task 8's derivation alone reached 37", covered)
	}
	if claudeTotal > 0 && claudeCovered != claudeTotal {
		t.Fatalf("%d of %d claude-format providers have no plan; that wire is the case draft 017 §4.6 names", claudeTotal-claudeCovered, claudeTotal)
	}
	t.Logf("plan covers %d entries, leaves %d uncovered (of %d with a transport)", covered, uncovered, covered+uncovered)
}

// TestPlanFor_GeminiIsItsOwnModelsPath pins the one gemini-format entry, so a
// registry change that moves its base is noticed here rather than as a probe that
// silently answers "no validation endpoint".
func TestPlanFor_GeminiIsItsOwnModelsPath(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	entry, ok := index.Provider("gemini")
	if !ok {
		t.Fatal("the gemini entry is not in the registry")
	}
	plan := PlanFor(entry)
	if plan.URL == "" || plan.Method != "GET" {
		t.Fatalf("gemini's plan = %+v, want a GET on its models path", plan)
	}
}
