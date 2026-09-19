// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_test.go
// @for       Table-driven tests for the settings defaults, partial patch, and
//
//	the deprecated caveman key.
//
// @uses      encoding/json, testing.
// @reason    AGENTS.md §2.1 and §2.4 require the defaults and the per-key
//
//	validation to be pinned, and SPEC-API-001 §7.9 makes the caveman key
//	a rule rather than a value: it stays accepted and frozen so an
//	exported reference configuration round-trips, and it is never
//	rendered. That rule needs a test or it is a comment.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"reflect"
	"testing"
)

// TestDefaultSettings pins every value SPEC-API-001 §7.14 documents, so a
// changed default is a failing test rather than a silent behaviour change.
func TestDefaultSettings(t *testing.T) {
	defaults := DefaultSettings()
	cases := []struct {
		name string
		got  any
		want any
	}{
		{"require_login", defaults.Security.RequireLogin, true},
		{"require_api_key", defaults.Security.RequireAPIKey, true},
		{"combo_strategy", string(defaults.Routing.ComboStrategy), "fallback"},
		{"combo_sticky_limit", defaults.Routing.ComboStickyLimit, 1},
		{"sticky_limit", defaults.Routing.StickyLimit, 3},
		{"outbound_proxy_enabled", defaults.Network.OutboundProxyEnabled, false},
		{"outbound_proxy_url", defaults.Network.OutboundProxyURL, ""},
		{"outbound_no_proxy", defaults.Network.OutboundNoProxy, ""},
		{"rtk disabled", defaults.TokenSaver.RTK.Enabled, false},
		{"rtk filters empty", defaults.TokenSaver.RTK.Filters, []string{}},
		{"headroom disabled", defaults.TokenSaver.Headroom.Enabled, false},
		{"ponytail disabled", defaults.TokenSaver.Ponytail.Enabled, false},
		{"request_capture_enabled", defaults.Logging.RequestCaptureEnabled, false},
		{"retention_days", defaults.Logging.RetentionDays, 7},
		{"capture_body_max_bytes", defaults.Logging.CaptureBodyMaxBytes, 65536},
		{"observability_max_records", defaults.Logging.ObservabilityMaxRecords, 1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !reflect.DeepEqual(tc.got, tc.want) {
				t.Fatalf("default %s = %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}

	if err := defaults.Validate(); err != nil {
		t.Fatalf("the documented defaults must satisfy their own rules: %v", err)
	}
}

// TestSettings_UpdateAppliesOnlyProvidedKeys covers the partial-PATCH contract
// at the boundaries: an empty patch changes nothing, a single key changes one
// key, and a zero value is applied rather than treated as absent.
func TestSettings_UpdateAppliesOnlyProvidedKeys(t *testing.T) {
	no := false
	zeroLimit := 0
	enabled := true
	captureOff := false

	cases := []struct {
		name           string
		patch          SettingsPatch
		wantErr        bool
		wantSticky     int
		wantRequireKey bool
		wantStrategy   ComboStrategy
		wantCaptureOn  bool
	}{
		{
			name:           "empty patch leaves everything",
			patch:          SettingsPatch{},
			wantSticky:     3,
			wantRequireKey: true,
			wantStrategy:   ComboFallback,
			wantCaptureOn:  false,
		},
		{
			name:           "one key in one group",
			patch:          SettingsPatch{Security: &SecuritySettingsPatch{RequireAPIKey: &no}},
			wantSticky:     3,
			wantRequireKey: false,
			wantStrategy:   ComboFallback,
			wantCaptureOn:  false,
		},
		{
			name:           "zero is applied, not treated as absent",
			patch:          SettingsPatch{Routing: &RoutingSettingsPatch{StickyLimit: &zeroLimit}},
			wantErr:        true,
			wantSticky:     3,
			wantRequireKey: true,
			wantStrategy:   ComboFallback,
			wantCaptureOn:  false,
		},
		{
			name:           "strategy and capture together",
			patch:          SettingsPatch{Routing: &RoutingSettingsPatch{ComboStrategy: strategyOf(ComboRoundRobin)}, Logging: &LoggingSettingsPatch{RequestCaptureEnabled: &enabled}},
			wantSticky:     3,
			wantRequireKey: true,
			wantStrategy:   ComboRoundRobin,
			wantCaptureOn:  true,
		},
		{
			name:           "changing capture back off",
			patch:          SettingsPatch{Logging: &LoggingSettingsPatch{RequestCaptureEnabled: &captureOff}},
			wantSticky:     3,
			wantRequireKey: true,
			wantStrategy:   ComboFallback,
			wantCaptureOn:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := DefaultSettings()
			err := settings.Update(tc.patch)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Update = nil, want a rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("Update error = %v", err)
			}
			if settings.Routing.StickyLimit != tc.wantSticky {
				t.Fatalf("sticky_limit = %d, want %d", settings.Routing.StickyLimit, tc.wantSticky)
			}
			if settings.Security.RequireAPIKey != tc.wantRequireKey {
				t.Fatalf("require_api_key = %v, want %v", settings.Security.RequireAPIKey, tc.wantRequireKey)
			}
			if settings.Routing.ComboStrategy != tc.wantStrategy {
				t.Fatalf("combo_strategy = %q, want %q", settings.Routing.ComboStrategy, tc.wantStrategy)
			}
			if settings.Logging.RequestCaptureEnabled != tc.wantCaptureOn {
				t.Fatalf("request_capture_enabled = %v, want %v", settings.Logging.RequestCaptureEnabled, tc.wantCaptureOn)
			}
		})
	}
}

// TestSettings_ValidateRejectsCrossKeyCombinations covers the rules a per-field
// check cannot see: headroom without a URL, and a bound below one.
func TestSettings_ValidateRejectsCrossKeyCombinations(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Settings)
	}{
		{"retention below one", func(s *Settings) { s.Logging.RetentionDays = 0 }},
		{"negative retention", func(s *Settings) { s.Logging.RetentionDays = -1 }},
		{"capture cap below one", func(s *Settings) { s.Logging.CaptureBodyMaxBytes = 0 }},
		{"observability below one", func(s *Settings) { s.Logging.ObservabilityMaxRecords = 0 }},
		{"headroom enabled without a url", func(s *Settings) { s.TokenSaver.Headroom.Enabled = true }},
		{"headroom url with a non-http scheme", func(s *Settings) {
			s.TokenSaver.Headroom.Enabled = true
			s.TokenSaver.Headroom.URL = "ftp://localhost:8787"
		}},
		{"headroom url without a host", func(s *Settings) {
			s.TokenSaver.Headroom.Enabled = true
			s.TokenSaver.Headroom.URL = "http://"
		}},
		{"headroom url that is not a URL", func(s *Settings) {
			s.TokenSaver.Headroom.Enabled = true
			s.TokenSaver.Headroom.URL = "not a url at all"
		}},
		{"a disabled headroom with a non-http url", func(s *Settings) {
			s.TokenSaver.Headroom.URL = "file:///etc/passwd"
		}},
		{"unknown strategy", func(s *Settings) { s.Routing.ComboStrategy = "fusion_v2" }},
		{"combo sticky below one", func(s *Settings) { s.Routing.ComboStickyLimit = 0 }},
		{"an rtk filter the engine does not implement", func(s *Settings) {
			s.TokenSaver.RTK.Filters = []string{"git-diff", "summarize"}
		}},
		{"an empty rtk filter name", func(s *Settings) {
			s.TokenSaver.RTK.Filters = []string{""}
		}},
		{"an rtk filter alias instead of the canonical name", func(s *Settings) {
			s.TokenSaver.RTK.Filters = []string{"rg"}
		}},
		{"an unknown ponytail level", func(s *Settings) { s.TokenSaver.Ponytail.Level = "maximum" }},
		{"an empty ponytail level", func(s *Settings) { s.TokenSaver.Ponytail.Level = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := DefaultSettings()
			tc.mutate(&settings)
			if err := settings.Validate(); err == nil {
				t.Fatalf("Validate = nil for %s, want a rejection", tc.name)
			}
		})
	}

	withURL := DefaultSettings()
	withURL.TokenSaver.Headroom.Enabled = true
	withURL.TokenSaver.Headroom.URL = "http://localhost:8787"
	if err := withURL.Validate(); err != nil {
		t.Fatalf("headroom with a url must be accepted: %v", err)
	}

	withFilters := DefaultSettings()
	withFilters.TokenSaver.RTK.Enabled = true
	withFilters.TokenSaver.RTK.Filters = append([]string{}, TokenSaverFilters...)
	if err := withFilters.Validate(); err != nil {
		t.Fatalf("every canonical filter must be accepted: %v", err)
	}
}

// TestValidTokenSaverFilter pins the vocabulary itself: the twelve names are the
// reference engine's, and nothing else is a configuration value. The count is
// asserted so a name added to the engine without a decision here is a failure.
func TestValidTokenSaverFilter(t *testing.T) {
	if len(TokenSaverFilters) != 12 {
		t.Fatalf("TokenSaverFilters carries %d names, want the twelve engine filters", len(TokenSaverFilters))
	}
	seen := map[string]bool{}
	for _, name := range TokenSaverFilters {
		if !ValidTokenSaverFilter(name) {
			t.Fatalf("ValidTokenSaverFilter(%q) = false for a listed name", name)
		}
		if seen[name] {
			t.Fatalf("TokenSaverFilters lists %q twice", name)
		}
		seen[name] = true
	}
	for _, rejected := range []string{"", "rg", "fd", "summarize", "git-diff ", "GIT-DIFF"} {
		if ValidTokenSaverFilter(rejected) {
			t.Fatalf("ValidTokenSaverFilter(%q) = true, want false", rejected)
		}
	}
}

// TestSettings_UpdateReplacesTheFilterAllowlist pins the RTK patch semantics: the
// list is a whole replacement, so a patch carrying an empty list clears the
// allowlist rather than leaving it alone, which the nil pointer already means.
func TestSettings_UpdateReplacesTheFilterAllowlist(t *testing.T) {
	on := true
	two := []string{"git-diff", "grep"}
	none := []string{}

	cases := []struct {
		name    string
		patch   *TokenSaverRTKPatch
		wantOn  bool
		wantLen int
	}{
		{name: "enabled with two filters", patch: &TokenSaverRTKPatch{Enabled: &on, Filters: &two}, wantOn: true, wantLen: 2},
		{name: "an empty list clears the allowlist", patch: &TokenSaverRTKPatch{Filters: &none}, wantOn: false, wantLen: 0},
		{name: "only the flag changes", patch: &TokenSaverRTKPatch{Enabled: &on}, wantOn: true, wantLen: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := DefaultSettings()
			settings.TokenSaver.RTK.Filters = []string{"ls", "tree"}
			if err := settings.Update(SettingsPatch{TokenSaver: &TokenSaverSettingsPatch{RTK: tc.patch}}); err != nil {
				t.Fatalf("Update error = %v", err)
			}
			if settings.TokenSaver.RTK.Enabled != tc.wantOn {
				t.Fatalf("enabled = %v, want %v", settings.TokenSaver.RTK.Enabled, tc.wantOn)
			}
			if len(settings.TokenSaver.RTK.Filters) != tc.wantLen {
				t.Fatalf("filters = %v, want %d entries", settings.TokenSaver.RTK.Filters, tc.wantLen)
			}
			if tc.wantLen == 0 && settings.TokenSaver.RTK.Filters == nil {
				t.Fatal("a cleared allowlist must be an empty list, never nil")
			}
		})
	}
}
