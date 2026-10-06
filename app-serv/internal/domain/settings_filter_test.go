// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_filter_test.go
// @for       The token-saver filter allowlist and the cross-key rules settings reject.
// @uses      reflect, testing.
// @reason    Both are settings-wide invariants: a PATCH replaces the allowlist rather than appending to it, and two individually valid keys can conflict with each other.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-04

package domain

import (
	"testing"
)

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
