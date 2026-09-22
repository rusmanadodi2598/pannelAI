// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_active.go
// @for       The in-flight marker: one request being routed right now, and the
//
//	wire codec the shared store reads it back through.
//
// @uses      internal/domain (NewULID), bytes, encoding/json, fmt, strings, time.
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
//	The store is a Redis instance other processes share, so a marker read
//	back is untrusted input (OWASP A08): the decoder re-applies every
//	invariant the constructor enforces rather than trusting the value.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-22
package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

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
	// StartedAt is the instant the call began, which the staleness window is
	// measured from.
	StartedAt time.Time
}

// NewActiveRequest validates and builds a marker for one call.
//
// The marker id is a ULID seeded with the start instant rather than the request
// id, because one request may be attempted against more than one provider: the
// combo failover path resolves a second member under the same request id, and
// two markers keyed by request id would overwrite each other so the first
// member's marker could never be removed.
func NewActiveRequest(requestID, providerID, endpointID, model string, now time.Time) (ActiveRequest, error) {
	marker := ActiveRequest{
		MarkerID:   NewULID(now),
		RequestID:  requestID,
		ProviderID: providerID,
		EndpointID: endpointID,
		Model:      model,
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

// activeRequestWire is the JSON shape the store carries. It is an explicit
// struct rather than a tagged version of ActiveRequest so a rename of a Go
// field cannot silently change what every already-stored marker decodes as.
type activeRequestWire struct {
	MarkerID   string `json:"marker_id"`
	RequestID  string `json:"request_id"`
	ProviderID string `json:"provider_id"`
	EndpointID string `json:"endpoint_id"`
	Model      string `json:"model"`
	StartedAt  string `json:"started_at"`
}

// EncodeActiveRequest renders a marker for the store, refusing one the
// aggregate would not have produced. Validating on the way in for the same
// reason the decoder validates on the way out: a marker that cannot be decoded
// is a value every later reader would have to drop.
func EncodeActiveRequest(marker ActiveRequest) ([]byte, error) {
	if err := marker.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(activeRequestWire{
		MarkerID:   marker.MarkerID,
		RequestID:  marker.RequestID,
		ProviderID: marker.ProviderID,
		EndpointID: marker.EndpointID,
		Model:      marker.Model,
		StartedAt:  marker.StartedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, fmt.Errorf("encoding active request: %w", err)
	}
	return payload, nil
}

// DecodeActiveRequest reads one stored marker back into a validated value.
//
// Every failure returns the zero marker alongside the error, so a caller cannot
// accidentally draw a half-decoded value. Unknown members are rejected rather
// than ignored: the value arrives from a Redis instance other processes share,
// so a member this build does not know is a contract change a reader has to be
// told about instead of silently dropping.
func DecodeActiveRequest(payload []byte) (ActiveRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()

	var wire activeRequestWire
	if err := decoder.Decode(&wire); err != nil {
		return ActiveRequest{}, fmt.Errorf("decoding active request: %w", err)
	}
	// Trailing content means the member is not one marker. json.Decoder stops
	// at the first value, so without this a member with a second object appended
	// would decode as if it were whole.
	if decoder.More() {
		return ActiveRequest{}, fmt.Errorf("decoding active request: trailing content after the marker")
	}
	startedAt, err := time.Parse(time.RFC3339Nano, wire.StartedAt)
	if err != nil {
		return ActiveRequest{}, fmt.Errorf("decoding active request: started_at is not an RFC3339 timestamp")
	}

	marker := ActiveRequest{
		MarkerID:   wire.MarkerID,
		RequestID:  wire.RequestID,
		ProviderID: wire.ProviderID,
		EndpointID: wire.EndpointID,
		Model:      wire.Model,
		StartedAt:  startedAt.UTC(),
	}
	if err := marker.Validate(); err != nil {
		return ActiveRequest{}, fmt.Errorf("decoding active request: %w", err)
	}
	return marker, nil
}
