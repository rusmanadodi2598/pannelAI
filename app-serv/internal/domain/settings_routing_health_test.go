// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing_health_test.go
// @for       The key-parking windows: their resolution per provider, the clamp that bounds a stored value, and the validation the write path enforces.
// @uses      testing, time.
// @reason    §7.14 makes the routing group the operator's only lever over how long a failing credential is parked, and §7.5 makes the router depend on the resolution agreeing with the panel's write. Draft 050 T8 is the finding that these were compile-time constants, so the numbers a deployment can now set are pinned here.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

import (
	"testing"
	"time"
)

// TestRoutingSettings_HealthForDefaults pins the floor: a document that names no
// window parks exactly as the constants did before the knobs existed.
func TestRoutingSettings_HealthForDefaults(t *testing.T) {
	settings := RoutingSettings{ProviderStrategies: map[string]ProviderStrategy{}}
	got := settings.HealthFor("openai")
	if want := DefaultKeyHealthPolicy(); got != want {
		t.Fatalf("HealthFor(unconfigured) = %+v, want %+v", got, want)
	}
	if got.AuthWindow != 2*time.Minute || got.RateLimitWindow != 2*time.Second || got.TransientWindow != 30*time.Second {
		t.Fatalf("documented windows changed: %+v", got)
	}
}

// TestRoutingSettings_HealthForRetunesEachClass checks that a group window reaches
// the class it names and leaves the other two at their defaults.
func TestRoutingSettings_HealthForRetunesEachClass(t *testing.T) {
	settings := RoutingSettings{
		KeyPark:            KeyParkWindows{AuthSeconds: 600},
		ProviderStrategies: map[string]ProviderStrategy{},
	}
	got := settings.HealthFor("openai")
	if got.AuthWindow != 10*time.Minute {
		t.Fatalf("auth window = %v, want the configured 10m", got.AuthWindow)
	}
	if got.RateLimitWindow != 2*time.Second || got.TransientWindow != 30*time.Second {
		t.Fatalf("untuned windows moved: %+v", got)
	}
}

// TestRoutingSettings_HealthForOverrideWinsPerWindow is the §7.14 rule one level
// down: a provider that retunes one window still inherits the group's other two,
// and an unknown provider inherits all three.
func TestRoutingSettings_HealthForOverrideWinsPerWindow(t *testing.T) {
	settings := RoutingSettings{
		KeyPark: KeyParkWindows{AuthSeconds: 600, TransientSeconds: 60},
		ProviderStrategies: map[string]ProviderStrategy{
			"gemini": {KeyPark: KeyParkWindows{TransientSeconds: 5}},
		},
	}
	got := settings.HealthFor("gemini")
	if got.TransientWindow != 5*time.Second {
		t.Fatalf("gemini transient = %v, want the provider's 5s", got.TransientWindow)
	}
	if got.AuthWindow != 10*time.Minute {
		t.Fatalf("gemini auth = %v, want the group's 10m", got.AuthWindow)
	}
	if other := settings.HealthFor("openai"); other.TransientWindow != 60*time.Second {
		t.Fatalf("openai transient = %v, want the group's 60s", other.TransientWindow)
	}
}

// TestKeyParkWindows_PolicyClamps guards the read path against a document written
// before the bounds existed: an absurd window is pulled to the edge rather than
// parked forever or dropped back to a default the operator did not ask for.
func TestKeyParkWindows_PolicyClamps(t *testing.T) {
	clamped := KeyParkWindows{AuthSeconds: 999999, RateLimitSeconds: -5}.Policy()
	if clamped.AuthWindow != time.Duration(MaxKeyParkAuthSeconds)*time.Second {
		t.Fatalf("auth window = %v, want the %ds ceiling", clamped.AuthWindow, MaxKeyParkAuthSeconds)
	}
	if clamped.RateLimitWindow != 2*time.Second {
		t.Fatalf("negative rate-limit window = %v, want the documented default", clamped.RateLimitWindow)
	}
}

// TestValidateRouting_KeyParkBounds is the write rule: an out-of-range window is a
// VALIDATION_ERROR at both levels, and a negative one cannot be stored at all.
func TestValidateRouting_KeyParkBounds(t *testing.T) {
	limit := 100
	for _, tc := range []struct {
		name    string
		windows KeyParkWindows
		entry   map[string]ProviderStrategy
		refuses bool
	}{
		{name: "group within bounds", windows: KeyParkWindows{AuthSeconds: MaxKeyParkAuthSeconds}},
		{name: "group past the ceiling", windows: KeyParkWindows{RateLimitSeconds: MaxKeyParkRateLimit + 1}, refuses: true},
		{name: "group negative", windows: KeyParkWindows{TransientSeconds: -1}, refuses: true},
		{
			name:    "provider past the ceiling",
			entry:   map[string]ProviderStrategy{"openai": {StickyLimit: &limit, KeyPark: KeyParkWindows{AuthSeconds: MaxKeyParkAuthSeconds + 1}}},
			refuses: true,
		},
	} {
		settings := defaultRoutingSettings()
		settings.KeyPark = tc.windows
		if tc.entry != nil {
			settings.ProviderStrategies = tc.entry
		}
		err := validateRouting(settings)
		if tc.refuses && err == nil {
			t.Fatalf("%s: validateRouting = nil error, want a rejection", tc.name)
		}
		if !tc.refuses && err != nil {
			t.Fatalf("%s: validateRouting error = %v", tc.name, err)
		}
	}
}

// TestValidateRouting_KeyParkAloneMakesAnEntryLive keeps the entry rule honest: an
// override that only retunes a window is real configuration, not a dead entry.
func TestValidateRouting_KeyParkAloneMakesAnEntryLive(t *testing.T) {
	settings := defaultRoutingSettings()
	settings.ProviderStrategies = map[string]ProviderStrategy{
		"openai": {KeyPark: KeyParkWindows{TransientSeconds: 45}},
	}
	if err := validateRouting(settings); err != nil {
		t.Fatalf("validateRouting(key_park only) error = %v, want the entry accepted", err)
	}
	dead := defaultRoutingSettings()
	dead.ProviderStrategies = map[string]ProviderStrategy{"openai": {}}
	if err := validateRouting(dead); err == nil {
		t.Fatal("validateRouting(empty entry) = nil error, want a rejection")
	}
}

// TestKeyParkWindows_MergedLeavesUnnamedWindows checks the per-field merge a
// provider override depends on, including a zero that means inherit rather than
// "set the window to nothing".
func TestKeyParkWindows_MergedLeavesUnnamedWindows(t *testing.T) {
	base := KeyParkWindows{AuthSeconds: 600, RateLimitSeconds: 10, TransientSeconds: 60}
	merged := base.Merged(KeyParkWindows{RateLimitSeconds: 30})
	if merged != (KeyParkWindows{AuthSeconds: 600, RateLimitSeconds: 30, TransientSeconds: 60}) {
		t.Fatalf("merged = %+v, want only rate_limit_seconds replaced", merged)
	}
	if !(KeyParkWindows{}).IsEmpty() {
		t.Fatal("an entry naming no window must read as empty")
	}
	if (KeyParkWindows{TransientSeconds: 1}).IsEmpty() {
		t.Fatal("an entry naming a window must not read as empty")
	}
}

// TestKeyParkPatch_AppliesOnlyWhatItNames is the PATCH rule: retuning one window
// leaves the stored value of the other two alone.
func TestKeyParkPatch_AppliesOnlyWhatItNames(t *testing.T) {
	stored := KeyParkWindows{AuthSeconds: 300, TransientSeconds: 45}
	seconds := 90
	(KeyParkPatch{AuthSeconds: &seconds}).applyTo(&stored)
	if stored != (KeyParkWindows{AuthSeconds: 90, TransientSeconds: 45}) {
		t.Fatalf("stored = %+v, want auth_seconds replaced and transient kept", stored)
	}
}
