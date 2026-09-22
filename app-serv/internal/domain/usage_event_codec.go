// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_event_codec.go
// @for       The wire form of the usage domain event: encode, and a decode that
//
//	re-validates before it returns.
//
// @uses      internal/domain (UsageEvent), encoding/json, fmt, strings, time.
// @reason    AGENTS.md §2.3 makes the event the seam between the recorder and
//
//	its consumers, and a broker is an untrusted input path: whatever can
//	publish to the channel can hand this process a payload. The codec is
//	here rather than in the repository so both directions of the
//	contract live beside the aggregate that defines its invariants, and
//	the decoder re-applies those invariants instead of trusting the
//	envelope (OWASP A08).
//
//	The wire form is an explicit struct rather than a tagged version of
//	UsageEvent: the event's fields are unexported-by-convention and the
//	JSON keys are snake_case like every other wire in this API, so a
//	rename in one place cannot silently change the channel's contract.
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

// usageEventWire is the JSON shape the channel carries. Every field is present
// on every payload, so a consumer never has to distinguish "absent" from
// "empty" and a truncated payload fails to decode rather than arriving
// half-populated.
type usageEventWire struct {
	Event       string `json:"event"`
	RequestID   string `json:"request_id"`
	ProviderID  string `json:"provider_id"`
	EndpointID  string `json:"endpoint_id"`
	Model       string `json:"model"`
	TotalTokens int64  `json:"total_tokens"`
	CostUSD     string `json:"cost_usd"`
	Status      string `json:"status"`
	OccurredAt  string `json:"occurred_at"`
}

// EncodeUsageEvent renders the event for the channel, refusing an event the
// aggregate would not have produced. The publisher half validates for the same
// reason the subscriber half does: a payload that cannot be decoded is a
// message a consumer would have to drop, and dropping it at the source keeps
// the channel's contents meaningful by construction.
func EncodeUsageEvent(event UsageEvent) ([]byte, error) {
	if err := event.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(usageEventWire{
		Event:       UsageEventName,
		RequestID:   event.RequestID,
		ProviderID:  event.ProviderID,
		EndpointID:  event.EndpointID,
		Model:       event.Model,
		TotalTokens: event.TotalTokens,
		CostUSD:     event.CostUSD,
		Status:      string(event.Status),
		OccurredAt:  event.OccurredAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, fmt.Errorf("encoding %s: %w", UsageEventName, err)
	}
	return payload, nil
}

// DecodeUsageEvent reads one channel payload back into an event.
//
// Every failure returns the zero event alongside the error, so a caller cannot
// accidentally act on a half-decoded value. The envelope is checked before the
// fields: a message that is not this event at all is refused as such, rather
// than reported as a missing request id, because the two need different
// responses from an operator (a misdelivery versus a malformed publisher).
//
// The decoder rejects unknown fields, so a payload carrying a field this build
// does not know is a contract change the consumer must be told about rather
// than one it silently ignores.
func DecodeUsageEvent(payload []byte) (UsageEvent, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()

	var wire usageEventWire
	if err := decoder.Decode(&wire); err != nil {
		return UsageEvent{}, fmt.Errorf("decoding %s: %w", UsageEventName, err)
	}
	// Trailing content means the payload is not one message. json.Decoder stops
	// at the first value, so without this check a payload with a second object
	// appended would decode as if it were whole.
	if decoder.More() {
		return UsageEvent{}, fmt.Errorf("decoding %s: trailing content after the event", UsageEventName)
	}
	if wire.Event != UsageEventName {
		return UsageEvent{}, fmt.Errorf("decoding %s: event name %q is not %q", UsageEventName, wire.Event, UsageEventName)
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, wire.OccurredAt)
	if err != nil {
		return UsageEvent{}, fmt.Errorf("decoding %s: occurred_at is not an RFC3339 timestamp", UsageEventName)
	}

	event := UsageEvent{
		RequestID:   wire.RequestID,
		ProviderID:  wire.ProviderID,
		EndpointID:  wire.EndpointID,
		Model:       wire.Model,
		TotalTokens: wire.TotalTokens,
		CostUSD:     wire.CostUSD,
		Status:      UsageStatus(wire.Status),
		OccurredAt:  occurredAt.UTC(),
	}
	if err := event.Validate(); err != nil {
		return UsageEvent{}, fmt.Errorf("decoding %s: %w", UsageEventName, err)
	}
	return event, nil
}

// SummaryLine renders the one console line a consumer writes for this event.
//
// The line is built from the event alone, so a consumer that only ever sees the
// channel can still say what happened, and every value it prints is one the
// event carries rather than something it looked up. The status is spelled out
// rather than echoed, because the field is a closed set and a reader should see
// which member of it this was.
func (e UsageEvent) SummaryLine() string {
	return fmt.Sprintf("usage %s: provider=%s endpoint=%s model=%s tokens=%d cost=%s status=%s",
		e.RequestID, e.ProviderID, e.EndpointID, e.Model, e.TotalTokens, e.CostUSD, e.Status)
}
