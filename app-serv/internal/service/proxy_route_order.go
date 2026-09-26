// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_route_order.go
// @for       The candidate order one proxy route walk tries: the operator's
//
//	stable order, the cooldown filter, rotation, and the provider's pin.
//
// @uses      internal/domain, context, sort.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D4-D6 and docs/PORT/009-PORT-
//
//	PROVIDER-PROXY.md D4 make the order a decision of its own, with its
//	own tests; it lives beside the plan rather than inside it so
//	proxy_route.go keeps one concern and stays under the file limit
//	(AGENTS.md §1.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package service

import (
	"context"
	"sort"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// candidateOrder returns the usable rows' ids in the order the walk should try
// them: the operator's stable order, minus parked candidates while an unparked
// one remains, rotated when the strategy is round-robin, with the provider's
// pinned row lifted to the front (D4-D6; PORT 009 D4).
//
// It sorts usable in place, so a caller that builds a by-id map from the same
// slice sees the order the ids come back in.
func (s *ProxyRouteService) candidateOrder(
	ctx context.Context,
	providerID string,
	binding domain.ProviderProxyBinding,
	usable []domain.Proxy,
) []string {
	// Stable order: insertion order is the operator's fallback order (D4).
	sort.SliceStable(usable, func(i, j int) bool {
		if !usable[i].CreatedAt().Equal(usable[j].CreatedAt()) {
			return usable[i].CreatedAt().Before(usable[j].CreatedAt())
		}
		return usable[i].ID() < usable[j].ID()
	})

	ids := make([]string, len(usable))
	for i, row := range usable {
		ids[i] = row.ID()
	}

	// Parked candidates sit out while an unparked one remains (D6); when every
	// usable candidate is parked the list stays whole, because a cooldown is a
	// hint about the past, not proof about the next dial.
	unparked := make([]string, 0, len(ids))
	for _, id := range ids {
		parked, err := s.routes.Parked(ctx, id)
		if err != nil {
			parked = false
		}
		if !parked {
			unparked = append(unparked, id)
		}
	}
	if len(unparked) > 0 {
		ids = unparked
	}

	if binding.Strategy == domain.ProxyStrategyRoundRobin {
		if rotated, err := s.routes.Next(ctx, domain.ProxyRoutePoolKeyFor(providerID), ids); err == nil {
			ids = rotated
		}
	}
	// The pin leads whatever order the strategy produced (D4). A pinned row
	// that is unusable, parked, or gone is simply absent from the list, so the
	// remaining candidates still serve.
	if binding.PoolID != "" {
		ids = moveIDToFront(ids, binding.PoolID)
	}
	return ids
}

// moveIDToFront lifts one id to the head of the list, leaving the order of the
// rest untouched. An id the list does not carry leaves it as it was.
func moveIDToFront(ids []string, id string) []string {
	at := -1
	for i, candidate := range ids {
		if candidate == id {
			at = i
			break
		}
	}
	if at <= 0 {
		return ids
	}
	reordered := make([]string, 0, len(ids))
	reordered = append(reordered, id)
	reordered = append(reordered, ids[:at]...)
	reordered = append(reordered, ids[at+1:]...)
	return reordered
}
