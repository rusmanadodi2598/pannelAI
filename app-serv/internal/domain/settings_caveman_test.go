// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_caveman_test.go
// @for       Tests pinning the DEPRECATED caveman saver key's contract.
// @uses      encoding/json, reflect, strings, testing.
// @reason    SPEC-API-001 §7.9 makes the caveman key a rule rather than a value:
//
//	it stays accepted and frozen so an exported reference configuration
//	round-trips, it is never rendered, and it is removed in /api/v2.
//	That rule needs a test, or "we did not add a control for it" is a
//	comment nobody can enforce.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestSettings_DeprecatedCavemanKeyRoundTrips pins the §7.9 rule: the caveman
// key is a stored key outside the rendered object, it keeps the documented
// default, it has no field on the renderable token-saver group, and the
// settings document the API returns does not carry it.
func TestSettings_DeprecatedCavemanKeyRoundTrips(t *testing.T) {
	// The key is in the stored set, so an exported configuration keeps it.
	if !containsSettingsKey(SettingsKeys, SettingsKeyCaveman) {
		t.Fatalf("SettingsKeys = %v, want it to contain the deprecated caveman key", SettingsKeys)
	}

	// The frozen default is the value §7.9 documents.
	frozen := DefaultCavemanSetting()
	if frozen.Enabled || frozen.Level != "full" {
		t.Fatalf("DefaultCavemanSetting() = %+v, want {false full}", frozen)
	}

	// A value read from storage round-trips unchanged, including one that a
	// client exported with the key switched on.
	stored := `{"enabled":true,"level":"ultra"}`
	var decoded CavemanSetting
	if err := json.Unmarshal([]byte(stored), &decoded); err != nil {
		t.Fatalf("decoding a stored caveman value: %v", err)
	}
	reencoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("encoding a stored caveman value: %v", err)
	}
	if string(reencoded) != stored {
		t.Fatalf("caveman round trip = %s, want %s", reencoded, stored)
	}

	// The rendered document has no caveman anywhere: not in the token-saver
	// group, and not at the top level.
	rendered, err := json.Marshal(DefaultSettings())
	if err != nil {
		t.Fatalf("encoding settings: %v", err)
	}
	if strings.Contains(string(rendered), "caveman") {
		t.Fatalf("settings response renders the deprecated key: %s", rendered)
	}

	// The patch type has no field that could carry it either.
	patchType := reflect.TypeOf(TokenSaverSettingsPatch{})
	for i := 0; i < patchType.NumField(); i++ {
		if strings.Contains(strings.ToLower(patchType.Field(i).Name), "caveman") {
			t.Fatalf("TokenSaverSettingsPatch carries a caveman field: %s", patchType.Field(i).Name)
		}
	}
}

// strategyOf returns a pointer to a strategy for a table case.
func strategyOf(strategy ComboStrategy) *ComboStrategy { return &strategy }

// containsSettingsKey reports whether the stored key set includes key.
func containsSettingsKey(keys []SettingsKey, key SettingsKey) bool {
	for _, candidate := range keys {
		if candidate == key {
			return true
		}
	}
	return false
}
