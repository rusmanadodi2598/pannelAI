// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_stub_bulk_test.go
// @for       The selection narrowing and the proxy pin roll-up of the in-memory endpoint store.
// @uses      context, internal/domain, internal/repository.
// @reason    The bulk services only behave correctly if this double honors the same filter predicates and the same reference roll-up as the real store: a double that ignored an id list or answered an empty pin report would let a selector test pass while production resolved a different set. Kept apart from the create-batch doubles so each stays inside the AGENTS.md §1.1 line budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ProxyPins answers the pins the real GROUP BY would find, by walking the stored
// endpoints: a bulk-delete test that asserted an empty report would pass against a store
// that ignored the very reference the refusal is about.
func (s *memEndpointStore) ProxyPins(_ context.Context, poolIDs []string) (map[string][]repository.ProxyPin, error) {
	out := make(map[string][]repository.ProxyPin, len(poolIDs))
	wanted := make(map[string]struct{}, len(poolIDs))
	for _, id := range poolIDs {
		wanted[id] = struct{}{}
	}
	for _, endpoint := range s.byID {
		pool := endpoint.ProxyPoolID()
		if _, ok := wanted[pool]; !ok {
			continue
		}
		pins := out[pool]
		found := false
		for i, pin := range pins {
			if pin.ProviderID == endpoint.ProviderID() {
				pins[i].Endpoints++
				found = true
				break
			}
		}
		if !found {
			pins = append(pins, repository.ProxyPin{ProviderID: endpoint.ProviderID(), Endpoints: 1})
		}
		out[pool] = pins
	}
	return out, nil
}

// containsID mirrors the real filter's `cardinality(...) = 0 OR id = ANY(...)`: the
// double has to narrow the same way or a selector test would pass against a store that
// ignores the set it was given.
func containsID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// holdsTag answers the containment the GIN index performs, on the canonical spelling the
// selector already carries.
func holdsTag(tags []domain.Tag, tag string) bool {
	for _, held := range tags {
		if held.String() == tag {
			return true
		}
	}
	return false
}
