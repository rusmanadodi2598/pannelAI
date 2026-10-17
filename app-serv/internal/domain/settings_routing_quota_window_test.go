// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing_quota_window_test.go
// @for       The provider reset cadence as routing policy: the quota ladder it caps, the operator park it sizes, and the seeded five-hour family.
// @uses      testing, time.
// @reason    F16 was a policy an operator could write and the ladder could not read, so the park length a panel configured silently stayed a compile-time number. This pins both readers of one field, the walk it bounds, and the case that matters operationally: a vendor that resets every five hours must never hold a usable credential for a day.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-16
package domain

import (
	"testing"
	"time"
)

func TestQuotaLadderHonoursTheConfiguredReset(t *testing.T) {
	cases := []struct {
		name    string
		policy  KeyHealthPolicy
		strikes int
		want    time.Duration
	}{
		{"no policy keeps the documented first step", KeyHealthPolicy{}, 1, 15 * time.Minute},
		{"the third rung is unchanged when no cap is set", KeyHealthPolicy{QuotaWindow: 24 * time.Hour}, 3, 6 * time.Hour},
		{"the last rung is a day when no cap is set", KeyHealthPolicy{QuotaWindow: 24 * time.Hour}, 4, 24 * time.Hour},
		{"a run past the ladder still stops at the cap", KeyHealthPolicy{QuotaWindow: 24 * time.Hour}, 9, 24 * time.Hour},
		{"a five-hour reset cuts the third rung", KeyHealthPolicy{QuotaWindow: 5 * time.Hour}, 3, 5 * time.Hour},
		{"a five-hour reset never exceeds the vendor's own period", KeyHealthPolicy{QuotaWindow: 5 * time.Hour}, 9, 5 * time.Hour},
		{"a short cap does not shorten a rung below it", KeyHealthPolicy{QuotaWindow: 5 * time.Hour}, 1, 15 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.policy.WindowFor(KeyFailureQuota, tc.strikes)
			if got != tc.want {
				t.Fatalf("WindowFor(quota, %d) = %v, want %v", tc.strikes, got, tc.want)
			}
		})
	}
}

func TestDefaultRoutingSeedsTheFiveHourFamily(t *testing.T) {
	settings := defaultRoutingSettings()

	fiveHour := []string{"openai", "claude", "gemini", "gemini-cli", "antigravity"}
	for _, id := range fiveHour {
		if got := settings.HealthFor(id).DisableWindow(); got != 5*time.Hour {
			t.Fatalf("HealthFor(%q).DisableWindow() = %v, want the five-hour reset", id, got)
		}
	}
	// A provider nobody seeded keeps the day, which is the cadence the free lanes
	// without a published shorter period actually run on.
	if got := settings.HealthFor("deepseek").DisableWindow(); got != DisabledWindowDefault {
		t.Fatalf("HealthFor(\"deepseek\").DisableWindow() = %v, want %v", got, DisabledWindowDefault)
	}
}

// TestOperatorOverrideBeatsTheSeed pins the point of keeping the cadence in the routing
// document rather than in code: the panel's write is what the router parks with, so a
// seeded default that an override could not displace would be a second path pretending
// policy existed.
func TestOperatorOverrideBeatsTheSeed(t *testing.T) {
	settings := defaultRoutingSettings()
	seconds := 600
	windows := settings.ProviderStrategies["gemini"].KeyPark
	KeyParkPatch{QuotaSeconds: &seconds}.applyTo(&windows)
	settings.ProviderStrategies["gemini"] = ProviderStrategy{KeyPark: windows}

	if got := settings.HealthFor("gemini").DisableWindow(); got != 10*time.Minute {
		t.Fatalf("DisableWindow() after the override = %v, want ten minutes", got)
	}
	if got := settings.HealthFor("gemini").WindowFor(KeyFailureQuota, 4); got != 10*time.Minute {
		t.Fatalf("the ladder's last rung = %v, want it capped by the same override", got)
	}
}

func TestKeyParkQuotaWindowValidatesAndMerges(t *testing.T) {
	if err := validateRouting(defaultRoutingSettings()); err != nil {
		t.Fatalf("the seeded document does not validate: %v", err)
	}

	rejected := defaultRoutingSettings()
	rejected.KeyPark = KeyParkWindows{QuotaSeconds: MaxKeyParkQuota + 1}
	if err := validateRouting(rejected); err == nil {
		t.Fatal("a quota window past the ladder's own ceiling was accepted")
	}

	// A provider that names only one window inherits the rest of the group's, which is
	// what makes a one-class retune possible without restating the others.
	group := KeyParkWindows{AuthSeconds: 300, QuotaSeconds: 18000}
	merged := group.Merged(KeyParkWindows{AuthSeconds: 60})
	if merged.AuthSeconds != 60 || merged.QuotaSeconds != 18000 {
		t.Fatalf("merged = %+v, want the override's auth and the group's quota", merged)
	}
	if !(KeyParkWindows{}).IsEmpty() {
		t.Fatal("an entry that names nothing was not read as empty")
	}
	if (KeyParkWindows{QuotaSeconds: 1}).IsEmpty() {
		t.Fatal("IsEmpty does not see the quota window, so a quota-only override would be stored as dead config")
	}
}
