// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_provider_proxy_test.go
// @for       The per-provider proxy binding: how it resolves against the global
//
//	setting and what the write path refuses to store.
//
// @uses      internal/domain (Settings, SettingsPatch, NetworkSettings), testing.
// @reason    docs/PORT/009-PORT-PROVIDER-PROXY.md D1-D3/D6: the binding decides
//
//	which pool one provider's calls walk, so both halves matter: the
//	resolution (override over global, `__none__` distinct from empty) and
//	the backstop that keeps a dead entry out of the document.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-26
package domain

import (
	"strings"
	"testing"
)

// TestNetworkSettings_ProviderProxyFor_ResolvesTheBinding pins the three modes
// and the strategy inheritance rule (D2/D3).
func TestNetworkSettings_ProviderProxyFor_ResolvesTheBinding(t *testing.T) {
	cases := []struct {
		name     string
		network  NetworkSettings
		provider string
		bound    bool
		poolID   string
		strategy string
	}{
		{
			name:     "no entry inherits the global setting",
			network:  NetworkSettings{OutboundProxyStrategy: ProxyStrategyRoundRobin},
			provider: "openai",
			bound:    false,
			poolID:   "",
			strategy: ProxyStrategyRoundRobin,
		},
		{
			name: "an entry without a pool stays global",
			network: NetworkSettings{
				OutboundProxyStrategy: ProxyStrategyFallback,
				ProviderProxies:       map[string]ProviderProxy{"openai": {Strategy: ProxyStrategyRoundRobin}},
			},
			provider: "openai",
			bound:    true,
			poolID:   "",
			strategy: ProxyStrategyRoundRobin,
		},
		{
			name: "a pool id is answered with the global strategy when the entry sets none",
			network: NetworkSettings{
				OutboundProxyStrategy: ProxyStrategyFallback,
				ProviderProxies:       map[string]ProviderProxy{"openai": {PoolID: "row-1"}},
			},
			provider: "openai",
			bound:    true,
			poolID:   "row-1",
			strategy: ProxyStrategyFallback,
		},
		{
			name: "the none sentinel is a value, not an absence",
			network: NetworkSettings{
				OutboundProxyStrategy: ProxyStrategyFallback,
				ProviderProxies:       map[string]ProviderProxy{"openai": {PoolID: ProxyPoolNone}},
			},
			provider: "openai",
			bound:    true,
			poolID:   ProxyPoolNone,
			strategy: ProxyStrategyFallback,
		},
		{
			name: "another provider's entry does not leak",
			network: NetworkSettings{
				OutboundProxyStrategy: ProxyStrategyFallback,
				ProviderProxies:       map[string]ProviderProxy{"openai": {PoolID: "row-1"}},
			},
			provider: "anthropic",
			bound:    false,
			poolID:   "",
			strategy: ProxyStrategyFallback,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binding, err := tc.network.ProviderProxyFor(tc.provider)
			if err != nil {
				t.Fatalf("ProviderProxyFor() = %v, want a resolved binding", err)
			}
			if binding.Bound != tc.bound {
				t.Fatalf("Bound = %v, want %v", binding.Bound, tc.bound)
			}
			if binding.PoolID != tc.poolID {
				t.Fatalf("PoolID = %q, want %q", binding.PoolID, tc.poolID)
			}
			if binding.Strategy != tc.strategy {
				t.Fatalf("Strategy = %q, want %q", binding.Strategy, tc.strategy)
			}
		})
	}
}

// TestNetworkSettings_ProviderProxyFor_RefusesABadStoredStrategy keeps a
// hand-edited row from routing every call of one provider: the resolution names
// the offending entry rather than failing silently or answering a default.
func TestNetworkSettings_ProviderProxyFor_RefusesABadStoredStrategy(t *testing.T) {
	network := NetworkSettings{
		OutboundProxyStrategy: ProxyStrategyFallback,
		ProviderProxies:       map[string]ProviderProxy{"openai": {Strategy: "random"}},
	}
	_, err := network.ProviderProxyFor("openai")
	if err == nil {
		t.Fatal("ProviderProxyFor() = nil error, want the out-of-set override refused")
	}
	if !strings.Contains(err.Error(), "provider_proxies[openai].strategy") {
		t.Fatalf("error = %q, want it to name the entry", err)
	}
}

// TestSettings_Update_ProviderProxyBackstop drives the whole-document write
// with raw patches, bypassing the schema tags: a member lands, and every dead
// entry is refused (D6).
func TestSettings_Update_ProviderProxyBackstop(t *testing.T) {
	settings := DefaultSettings()
	if err := settings.Update(SettingsPatch{Network: &NetworkSettingsPatch{
		ProviderProxies: &map[string]ProviderProxy{
			"openai":    {PoolID: ProxyPoolNone},
			"anthropic": {PoolID: "row-1", Strategy: ProxyStrategyRoundRobin},
		},
	}}); err != nil {
		t.Fatalf("Update() = %v, want the entries stored", err)
	}
	if got := settings.Network.ProviderProxies["anthropic"].Strategy; got != ProxyStrategyRoundRobin {
		t.Fatalf("stored strategy = %q, want %q", got, ProxyStrategyRoundRobin)
	}

	refused := []struct {
		name  string
		entry ProviderProxy
		want  string
	}{
		{
			name:  "an entry that sets nothing is dead configuration",
			entry: ProviderProxy{},
			want:  "must set pool_id or strategy",
		},
		{
			name:  "an out-of-set strategy is refused",
			entry: ProviderProxy{Strategy: "random"},
			want:  "strategy must be fallback or round_robin",
		},
		{
			name:  "an over-long pool id is refused",
			entry: ProviderProxy{PoolID: strings.Repeat("x", 65)},
			want:  "pool_id must be at most 64 characters",
		},
		{
			name:  "a whitespace pool id is not an id",
			entry: ProviderProxy{PoolID: " row-1 "},
			want:  "pool_id must not carry surrounding whitespace",
		},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			err := settings.Update(SettingsPatch{Network: &NetworkSettingsPatch{
				ProviderProxies: &map[string]ProviderProxy{"openai": tc.entry},
			}})
			if err == nil {
				t.Fatal("Update() = nil error, want the entry refused")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestSettings_Update_ProviderProxiesReplacesTheMap pins the whole-map rule: an
// empty map is a real value (clear every binding), which the nil pointer cannot
// express.
func TestSettings_Update_ProviderProxiesReplacesTheMap(t *testing.T) {
	settings := DefaultSettings()
	if err := settings.Update(SettingsPatch{Network: &NetworkSettingsPatch{
		ProviderProxies: &map[string]ProviderProxy{"openai": {PoolID: "row-1"}},
	}}); err != nil {
		t.Fatalf("Update() = %v, want the entry stored", err)
	}
	if err := settings.Update(SettingsPatch{Network: &NetworkSettingsPatch{
		ProviderProxies: &map[string]ProviderProxy{},
	}}); err != nil {
		t.Fatalf("Update() = %v, want the empty map accepted", err)
	}
	if len(settings.Network.ProviderProxies) != 0 {
		t.Fatalf("ProviderProxies = %v, want the map cleared", settings.Network.ProviderProxies)
	}
}
