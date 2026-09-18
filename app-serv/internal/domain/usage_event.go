// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_event.go
// @for       The usage domain event a recorded request publishes so quota and
//
//	log consumers can react without a synchronous call.
//
// @uses      internal/domain (UsageRecord, UsageStatus), time.
// @reason    AGENTS.md §2.3 requires a mutation on an aggregate root to emit an
//
//	event; the payload is the routing identity and the counters a
//	quota flush needs, which is deliberately less than the whole
//	record, so a consumer cannot come to depend on fields the event
//	does not carry.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import "time"

// UsageEventName is the domain event a recorded request publishes so quota and
// log consumers can react without a synchronous call (AGENTS.md §2.3).
const UsageEventName = "usage.recorded"

// UsageEvent is the payload of that event. It carries the routing identity and
// the counters a quota flush needs, not the whole record.
type UsageEvent struct {
	RequestID   string
	ProviderID  string
	EndpointID  string
	Model       string
	TotalTokens int64
	CostUSD     string
	Status      UsageStatus
	OccurredAt  time.Time
}

// NewUsageEvent derives the event from the record.
func (r UsageRecord) NewUsageEvent() UsageEvent {
	return UsageEvent{
		RequestID:   r.requestID,
		ProviderID:  r.providerID,
		EndpointID:  r.endpointID,
		Model:       r.model,
		TotalTokens: r.TotalTokens(),
		CostUSD:     r.costUSD,
		Status:      r.status,
		OccurredAt:  r.ts,
	}
}

// ErrUsageRecordNotFound is the sentinel a missing accounting row maps to, so
// the handler returns NOT_FOUND rather than a driver message (AGENTS.md §1.3).
var ErrUsageRecordNotFound = NewNotFoundError("usage record not found")
