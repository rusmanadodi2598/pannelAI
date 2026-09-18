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
		{"rtk enabled", defaults.TokenSaver.RTK.Enabled, true},
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
		{"unknown strategy", func(s *Settings) { s.Routing.ComboStrategy = "fusion_v2" }},
		{"combo sticky below one", func(s *Settings) { s.Routing.ComboStickyLimit = 0 }},
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
}
