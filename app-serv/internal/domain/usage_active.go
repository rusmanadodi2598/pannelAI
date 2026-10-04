// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_active.go
// @for       The in-flight marker: one request the gateway is routing right now.
//
// @uses      internal/domain (NewULID), time.
// @reason    SPEC-UI-001 §6.5 makes the drawing's "a provider is routing now"
//
//	claim come from a live stream, and a claim needs a source that
//	disappears when the thing it describes does: the marker is written
//	before the outbound call and removed after it, so the set of active
//	providers is what this process is actually doing rather than what it
//	once did. The 60 second window is the second half of that rule, and
//	it is the panel's own guard figure: a gateway that dies mid-request
//	never removes its marker, so a read that trusted the set forever
//	would light a node for a request that no longer exists (R-36).
//
//	The marker names the combo a client addressed as well as the provider
//	that answered, because SPEC-UI-001 §6.5 draws the request path and a
//	combo is the first stage of it. Its wire codec is a separate file
//	for the same reason the tests are split: this aggregate is the
//	routing fact, the codec is a stored value crossing a shared Redis.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-22
package domain

import "time"

// ActiveRequestStaleAfter is how long a marker stays eligible after the instant
// it says it started. It matches the panel's own guard so one figure governs
// both halves of the rule, and it is the ceiling on how long a marker from a
// process that died can still be drawn.
const ActiveRequestStaleAfter = 60 * time.Second

// ActiveRequest is one request the gateway is routing right now. Its fields are
// exported because the live read maps it onto a wire frame, which is the one
// place this aggregate is read from outside its own package.
type ActiveRequest struct {
	// MarkerID identifies this marker among every other one, so the removal
	// after a call cannot delete a different request's marker.
	MarkerID string
	// RequestID is the router's id for the call, so a reader can join the
	// marker to the usage row the same call eventually writes.
	RequestID string
	// ProviderID is the node the drawing lights. It is required: a marker with
	// no provider would light nothing and say nothing.
	ProviderID string
	// EndpointID and Model are optional context. A media call may resolve no
	// model, and the marker must still be writable for it.
	EndpointID string
	Model      string
	// Combo is the model combo the client addressed, or "" when the request
	// addressed a single model. It travels beside Model rather than replacing
	// it: the member is what answered and the combo is what the client named,
	// and the drawing lights both. Only the chat plane resolves a combo — the
	// media, embeddings and SystemOne planes refuse one before they mark
	// anything — so an empty Combo is the normal value, not a missing one.
	Combo string
	// StartedAt is the instant the call began, which the staleness window is
	// measured from.
	StartedAt time.Time
}

// ActiveRequestInput is one call's identity as the tracker presents it, before
// the marker id and start instant are added.
//
// A struct rather than a positional list because Model and Combo are both
// optional adjacent strings: a transposition compiles, passes Validate(), is
// stored, and draws the wrong label on the panel. UsageFilterInput and
// RequestLogInput are the same shape for the same reason.
type ActiveRequestInput struct {
	RequestID  string
	ProviderID string
	EndpointID string
	Model      string
	Combo      string
}

// NewActiveRequest validates and builds a marker for one call.
//
// The marker id is a ULID seeded with the start instant rather than the request
// id, because one request may be attempted against more than one provider: the
// combo failover path resolves a second member under the same request id, and
// two markers keyed by request id would overwrite each other so the first
// member's marker could never be removed.
func NewActiveRequest(in ActiveRequestInput, now time.Time) (ActiveRequest, error) {
	marker := ActiveRequest{
		MarkerID:   NewULID(now),
		RequestID:  in.RequestID,
		ProviderID: in.ProviderID,
		EndpointID: in.EndpointID,
		Model:      in.Model,
		Combo:      in.Combo,
		StartedAt:  now,
	}
	if err := marker.Validate(); err != nil {
		return ActiveRequest{}, err
	}
	return marker, nil
}

// Validate re-applies the marker's invariants to a value read back from the
// store, so a consumer can only ever act on a marker this process would have
// written.
func (a ActiveRequest) Validate() error {
	if a.MarkerID == "" {
		return NewValidationError("marker_id is required")
	}
	if a.RequestID == "" {
		return NewValidationError("request_id is required")
	}
	if a.ProviderID == "" {
		return NewValidationError("provider_id is required")
	}
	if a.StartedAt.IsZero() {
		return NewValidationError("started_at is required")
	}
	return nil
}

// Score is the ordering key the store holds the marker under: milliseconds
// since the epoch, so the set is read oldest first and a bounded range read
// needs no sort of its own.
func (a ActiveRequest) Score() float64 { return float64(a.StartedAt.UnixMilli()) }

// ActiveRequestCutoff is the oldest instant a read at now still treats as live.
// A marker at exactly this instant is already stale, which is the same
// exclusive boundary the panel's guard applies, so the two cannot disagree
// about a request sitting on the boundary.
func ActiveRequestCutoff(now time.Time) time.Time {
	return now.Add(-ActiveRequestStaleAfter)
}
