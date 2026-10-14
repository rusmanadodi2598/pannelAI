// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_fixtures_stamp_test.go
// @for       The in-memory endpoint repository's selection stamp, applied through the aggregate's own method.
// @uses      context, time, internal/domain.
// @reason    The stamp is the fact rotation orders by when Redis cannot supply a cursor, so the double has to advance it the way a real row does or a degraded-path test would prove nothing. Kept apart from the fixture's core so neither file crowds the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// MarkSelected records the stamp and applies it to the stored key, so a second
// selection sees the same ordering advance a real row would.
func (r *memEndpointRepo) MarkSelected(_ context.Context, endpointID, keyID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stampsSelected++
	if r.selectErr != nil {
		return r.selectErr
	}
	r.selected[endpointID+"/"+keyID] = at
	for pi, endpoint := range r.byProvider {
		for ei, stored := range endpoint {
			if stored.ID() != endpointID {
				continue
			}
			r.byProvider[pi][ei] = stampCandidate(stored, keyID, at)
		}
	}
	return nil
}

// stampCandidate re-stamps the chosen credential inside an endpoint, through the
// aggregate's own method rather than by editing stored fields.
func stampCandidate(endpoint domain.UpstreamEndpoint, keyID string, at time.Time) domain.UpstreamEndpoint {
	if keyID == "" {
		endpoint.MarkSelected(at)
		return endpoint
	}
	if _, err := endpoint.RecordKeySelected(keyID, at); err != nil {
		return endpoint
	}
	return endpoint
}
