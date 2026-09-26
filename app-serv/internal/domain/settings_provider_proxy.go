// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_provider_proxy.go
// @for       The per-provider proxy binding: which pool one provider's calls
//
//	egress through, and the strategy that orders the walk.
//
// @uses      internal/domain (errors, proxy strategy), strings.
// @reason    docs/PORT/009-PORT-PROVIDER-PROXY.md D1-D3: the binding is one map
//
//	in the settings document keyed by provider id, mirroring the
//	reference's providerStrategies and this port's own
//	routing.provider_strategies, and the resolution rule (override over
//	global, `__none__` distinct from empty) lives here so the read, the
//	write, and the route plan cannot drift.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-26
package domain

import "strings"

// ProxyPoolNone is the stored spelling of "this provider dials direct" (D2).
// It follows the reference's sentinel. It exists because an empty pool id
// already means "inherit the global setting": without a distinct spelling,
// turning the global proxy on would leave an operator no way to exempt one
// provider, and a provider that must not be proxied (a local model server, an
// upstream that rejects proxied traffic) would have no lever.
const ProxyPoolNone = "__none__"

// The pool id's cap is maxProxyPoolIDLength (upstream_endpoint_parity.go), so
// one pool id is the same value whichever surface carries it.

// ProviderProxy is one provider's binding: the pool its calls egress through
// and the strategy that orders the walk. Both fields are optional and an
// absent one inherits the global setting, so the panel renders what is stored
// rather than a filled-in copy.
//
// An entry that sets neither field is refused by validateProviderProxies: it
// would be configuration nothing reads, and the way to express "inherit
// everything" is to delete the entry.
type ProviderProxy struct {
	// PoolID is "" (follow the global outbound setting), ProxyPoolNone (dial
	// direct), or the id of a stored pool row to pin first (D2/D4).
	PoolID string `json:"pool_id,omitempty"`
	// Strategy is "" (inherit network.outbound_proxy_strategy), fallback, or
	// round_robin (D3).
	Strategy string `json:"strategy,omitempty"`
}

// ProviderProxyBinding is the binding resolved for one provider: what the
// route plan reads, with every inherited value already filled in.
type ProviderProxyBinding struct {
	// Bound reports whether the provider has an entry at all. It is what tells
	// "the operator chose Global" from "the operator never said".
	Bound bool
	// PoolID is the stored pool id: "" follows the global setting, ProxyPoolNone
	// dials direct, and any other value names a stored row.
	PoolID string
	// Strategy is the effective strategy: the entry's override, or the global
	// one, and never empty.
	Strategy string
}

// Normalized fills the keys a stored row written before this change cannot
// carry. The group decode replaces the group wholesale, so an absent
// provider_proxies arrives as nil, and a read that answered null would fail the
// panel's own form validation: an operator with no bindings must still read an
// empty map. It mirrors RoutingSettings.Normalized.
func (n NetworkSettings) Normalized() NetworkSettings {
	if n.ProviderProxies == nil {
		n.ProviderProxies = map[string]ProviderProxy{}
	}
	return n
}

// ProviderProxyFor resolves the binding for one provider (D1-D3). The override
// wins over the global strategy; an entry whose strategy is out of the closed
// set is an error naming the entry, because a hand-edited row must refuse the
// plan rather than silently route by a rule the operator did not choose.
func (n NetworkSettings) ProviderProxyFor(providerID string) (ProviderProxyBinding, error) {
	entry, bound := n.ProviderProxies[providerID]
	binding := ProviderProxyBinding{Bound: bound}
	if bound {
		binding.PoolID = strings.TrimSpace(entry.PoolID)
	}
	strategy := n.OutboundProxyStrategy
	if bound && strings.TrimSpace(entry.Strategy) != "" {
		strategy = strings.TrimSpace(entry.Strategy)
	}
	parsed, err := ParseProxyStrategy(strategy)
	if err != nil {
		if bound && strings.TrimSpace(entry.Strategy) != "" {
			return ProviderProxyBinding{}, NewValidationError(
				"provider_proxies[" + providerID + "].strategy must be fallback or round_robin")
		}
		return ProviderProxyBinding{}, err
	}
	binding.Strategy = parsed
	return binding, nil
}

// validateProviderProxies checks the binding map's rules: every entry must say
// something, the strategy must be a member of the closed set, and the pool id
// must be an id rather than a padded string. Whether the id names a stored row
// is the service's check, because only it may read the pool repository
// (AGENTS.md §1.5).
func validateProviderProxies(entries map[string]ProviderProxy) error {
	for id, entry := range entries {
		if strings.TrimSpace(entry.PoolID) == "" && strings.TrimSpace(entry.Strategy) == "" {
			return NewValidationError("network.provider_proxies[" + id +
				"] must set pool_id or strategy; delete the entry to inherit the global setting")
		}
		if entry.PoolID != strings.TrimSpace(entry.PoolID) {
			return NewValidationError("network.provider_proxies[" + id +
				"].pool_id must not carry surrounding whitespace")
		}
		if len(entry.PoolID) > maxProxyPoolIDLength {
			return NewValidationError("network.provider_proxies[" + id +
				"].pool_id must be at most 64 characters")
		}
		if entry.Strategy != "" {
			if _, err := ParseProxyStrategy(entry.Strategy); err != nil {
				return NewValidationError("network.provider_proxies[" + id +
					"].strategy must be fallback or round_robin")
			}
		}
	}
	return nil
}
