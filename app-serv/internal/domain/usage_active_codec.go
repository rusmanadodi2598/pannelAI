// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_active_codec.go
// @for       The in-flight marker's wire codec: the JSON the shared store keeps.
// @uses      internal/domain (ActiveRequest), bytes, encoding/json, fmt, time.
// @reason    The store is a Redis instance other processes share, so a marker read back is untrusted input (OWASP A08): the decoder re-applies every invariant the constructor enforces rather than trusting the value. The codec is kept out of the aggregate's own file because the two answer different questions, which call is running, and how a running call survives a process boundary, and because a member this build does not know is a contract change a reader has to be told about, which is a rule about the wire rather than about routing.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-03
package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// activeRequestWire is the JSON shape the store carries. It is an explicit
// struct rather than a tagged version of ActiveRequest so a rename of a Go
// field cannot silently change what every already-stored marker decodes as.
//
// `combo` is omitted when empty so a marker for a call that addressed a single
// model, every media, embeddings and SystemOne call, and most chat requests,
// encodes byte-for-byte as the shape an older build writes.
type activeRequestWire struct {
	MarkerID   string `json:"marker_id"`
	RequestID  string `json:"request_id"`
	ProviderID string `json:"provider_id"`
	EndpointID string `json:"endpoint_id"`
	Model      string `json:"model"`
	Combo      string `json:"combo,omitempty"`
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
		Combo:      marker.Combo,
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
		Combo:      wire.Combo,
		StartedAt:  startedAt.UTC(),
	}
	if err := marker.Validate(); err != nil {
		return ActiveRequest{}, fmt.Errorf("decoding active request: %w", err)
	}
	return marker, nil
}
