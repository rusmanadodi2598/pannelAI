// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/endpoint_counts.go
// @for       The per-provider roll-up of stored upstream endpoints by state.
// @uses      internal/domain (UpstreamEndpointStatus).
// @reason    SPEC-API-001 §7.4 publishes a status_summary per provider so the
//
//	list screen does not fetch every account. The roll-up is a value object
//	rather than a schema struct because the count is measured against the
//	status vocabulary this package owns: a state added here must appear in the
//	summary, and deriving it in the schema layer would let the two drift.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

// EndpointStatusCounts is how many endpoints one provider holds in each state.
// Total is the sum of the other four and is carried rather than recomputed, so a
// caller rendering "3 of 10 healthy" reads two fields instead of adding four.
type EndpointStatusCounts struct {
	Total       int64
	Active      int64
	Disabled    int64
	Error       int64
	RateLimited int64
}

// Add folds one grouped measurement into the roll-up: how many endpoints hold
// one state, and whether they are currently rate-limited.
//
// A rate-limited endpoint is counted by its own state and not also as Active, so
// the four state counts sum to Total and a reader cannot double-count one
// account.
func (c *EndpointStatusCounts) Add(status UpstreamEndpointStatus, rateLimited bool, count int64) {
	if count <= 0 {
		return
	}
	c.Total += count
	if rateLimited {
		c.RateLimited += count
		return
	}
	switch status {
	case UpstreamEndpointActive:
		c.Active += count
	case UpstreamEndpointDisabled:
		c.Disabled += count
	default:
		c.Error += count
	}
}
