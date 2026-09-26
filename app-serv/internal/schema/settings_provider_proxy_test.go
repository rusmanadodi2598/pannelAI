// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_provider_proxy_test.go
// @for       The per-provider proxy binding on the wire: the tag layer, the
//
//	lowering into the domain patch, and the read shape.
//
// @uses      encoding/json, testing, internal/domain.
// @reason    docs/PORT/009-PORT-PROVIDER-PROXY.md D6/D10: the binding is a map
//
//	of dynamic keys, so the tags must reach into the entries (dive) and
//	the read must render an object rather than null, or the panel's own
//	form validation fails on a document with no bindings.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-26
package schema

import (
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestValidateStruct_GuardsTheProviderProxyValues pins the tag layer: an
// out-of-set strategy and an over-long pool id are refused inside a map entry,
// which is what the dive tag is for.
func TestValidateStruct_GuardsTheProviderProxyValues(t *testing.T) {
	valid := PatchSettingsRequest{Network: &NetworkSettingsPatch{
		ProviderProxies: &map[string]ProviderProxyPatch{
			"openai": {PoolID: strPtr("prx_a"), Strategy: strPtr(domain.ProxyStrategyRoundRobin)},
		},
	}}
	if err := ValidateStruct(valid); err != nil {
		t.Fatalf("a well-formed binding patch must be accepted: %v", err)
	}

	longID := make([]byte, 65)
	for i := range longID {
		longID[i] = 'x'
	}
	cases := []struct {
		name  string
		patch PatchSettingsRequest
	}{
		{"an out-of-set strategy", PatchSettingsRequest{Network: &NetworkSettingsPatch{
			ProviderProxies: &map[string]ProviderProxyPatch{"openai": {Strategy: strPtr("random")}},
		}}},
		{"an over-long pool id", PatchSettingsRequest{Network: &NetworkSettingsPatch{
			ProviderProxies: &map[string]ProviderProxyPatch{"openai": {PoolID: strPtr(string(longID))}},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateStruct(tc.patch); err == nil {
				t.Fatalf("ValidateStruct = nil for %q, want a rejection", tc.name)
			}
		})
	}
}

// TestToSettingsPatch_LowersTheProviderProxyMap pins the boundary translation:
// the map arrives whole so the aggregate replaces rather than merges, and an
// absent group lowers to nil.
func TestToSettingsPatch_LowersTheProviderProxyMap(t *testing.T) {
	entries := map[string]ProviderProxyPatch{
		"openai":    {PoolID: strPtr(domain.ProxyPoolNone)},
		"anthropic": {Strategy: strPtr(domain.ProxyStrategyRoundRobin)},
	}
	patch := ToSettingsPatch(PatchSettingsRequest{Network: &NetworkSettingsPatch{ProviderProxies: &entries}})

	if patch.Network == nil {
		t.Fatal("network patch = nil, want the lowered group")
	}
	if patch.Network.ProviderProxies == nil || len(*patch.Network.ProviderProxies) != 2 {
		t.Fatalf("provider_proxies = %v, want the two lowered entries", patch.Network.ProviderProxies)
	}
	if got := (*patch.Network.ProviderProxies)["openai"].PoolID; got != domain.ProxyPoolNone {
		t.Fatalf("openai pool_id = %q, want the sentinel", got)
	}
	if got := (*patch.Network.ProviderProxies)["anthropic"].Strategy; got != domain.ProxyStrategyRoundRobin {
		t.Fatalf("anthropic strategy = %q, want round_robin", got)
	}

	if ToSettingsPatch(PatchSettingsRequest{}).Network != nil {
		t.Fatal("an absent network group must lower to nil, not to an empty patch")
	}
}

// TestSettingsResponseFrom_CarriesTheProviderProxyKeys pins the read shape: an
// empty binding map renders as {} rather than null, and an entry renders only
// the fields it stores.
func TestSettingsResponseFrom_CarriesTheProviderProxyKeys(t *testing.T) {
	empty := domain.Settings{Network: domain.NetworkSettings{
		OutboundProxyStrategy: domain.ProxyStrategyFallback,
		ProviderProxies:       map[string]domain.ProviderProxy{},
	}}
	body, err := json.Marshal(SettingsResponseFrom(empty).Network)
	if err != nil {
		t.Fatalf("marshalling the network group: %v", err)
	}
	var rendered map[string]json.RawMessage
	if err := json.Unmarshal(body, &rendered); err != nil {
		t.Fatalf("decoding the network group: %v", err)
	}
	if string(rendered["provider_proxies"]) != "{}" {
		t.Fatalf("provider_proxies = %s, want {}", rendered["provider_proxies"])
	}

	bound := domain.Settings{Network: domain.NetworkSettings{
		OutboundProxyStrategy: domain.ProxyStrategyFallback,
		ProviderProxies: map[string]domain.ProviderProxy{
			"openai": {PoolID: "prx_a"},
		},
	}}
	mapped := SettingsResponseFrom(bound).Network.ProviderProxies["openai"]
	if mapped.PoolID != "prx_a" {
		t.Fatalf("pool_id = %q, want the stored row", mapped.PoolID)
	}
	if mapped.Strategy != "" {
		t.Fatalf("strategy = %q, want an absent override to stay absent on the wire", mapped.Strategy)
	}
}
