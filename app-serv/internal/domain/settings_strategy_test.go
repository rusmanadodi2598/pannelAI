// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_strategy_test.go
// @for       The settings write path's strategy backstop: an out-of-set value
//
//	is refused before it is stored, whichever path carried it.
//
// @uses      internal/domain (Settings, SettingsPatch), testing.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D2: a stored strategy the plan
//
//	cannot parse would refuse every proxied request the engine routes,
//	so the write paths must never land one. The schema tag is the first
//	gate; Settings.Update is the class-level backstop the §7.14 PATCH
//	and any future writer share, and this file pins it without the tag.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-26
package domain

import "testing"

// TestSettings_Update_StrategyBackstop drives the whole-document write with a
// raw patch, bypassing the schema layer: a member lands, and an out-of-set
// value is refused with the parse error naming the key.
func TestSettings_Update_StrategyBackstop(t *testing.T) {
	settings := DefaultSettings()

	if err := settings.Update(SettingsPatch{Network: &NetworkSettingsPatch{
		OutboundProxyStrategy: strPtrStrategy(ProxyStrategyRoundRobin),
	}}); err != nil {
		t.Fatalf("Update() = %v, want the member stored", err)
	}
	if got := settings.Network.OutboundProxyStrategy; got != ProxyStrategyRoundRobin {
		t.Fatalf("stored strategy = %q, want %q", got, ProxyStrategyRoundRobin)
	}

	for _, bad := range []string{"random", "Fallback", "roundrobin"} {
		if err := settings.Update(SettingsPatch{Network: &NetworkSettingsPatch{
			OutboundProxyStrategy: strPtrStrategy(bad),
		}}); err == nil {
			t.Fatalf("Update(%q) = nil error, want the out-of-set value refused", bad)
		}
	}
}

// strPtrStrategy is this file's string pointer helper.
func strPtrStrategy(value string) *string { return &value }
