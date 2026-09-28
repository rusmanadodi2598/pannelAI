// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_envelope_frame.go
// @for       One Qoder SSE frame: its envelope, the answer wrapped inside it, and
//
//	what a refusal inside that envelope becomes.
//
// @uses      bytes, encoding/json, errors, fmt, net/http, strings, time.
// @reason    The vendor states a call's real outcome per frame rather than per
//
//	response, so parsing one frame and deciding what a refusal means are the
//	parts worth reading on their own; the reader that applies them to a
//	stream lives beside this file.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// qoderEnvelope is one frame's wrapper. `body` is the answer as a JSON string on the
// wire, and an object in some shapes, so both are read.
type qoderEnvelope struct {
	StatusCodeValue json.Number     `json:"statusCodeValue"`
	StatusCode      string          `json:"statusCode"`
	Body            json.RawMessage `json:"body"`
}

// qoderErrorChunk is the terminal frame a stream gets when the provider refuses after
// the answer started. The status the client already received cannot change, so the
// refusal is delivered as the answer's own content and finish — the shape the
// reference uses, in a typed form because the gateway does not build wire shapes out
// of untyped maps (AGENTS.md §1.4).
type qoderErrorChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Choices []qoderErrorChoice `json:"choices"`
}

type qoderErrorChoice struct {
	Index        int             `json:"index"`
	Delta        qoderErrorDelta `json:"delta"`
	FinishReason string          `json:"finish_reason"`
}

type qoderErrorDelta struct {
	Content string `json:"content"`
}

// parseQoderEnvelope decodes one frame's wrapper. An unparseable payload is reported
// as not-an-envelope so the caller can pass the answer through unchanged rather than
// drop a frame whose shape the vendor changed.
func parseQoderEnvelope(raw []byte) (qoderEnvelope, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return qoderEnvelope{}, false
	}
	var envelope qoderEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return qoderEnvelope{}, false
	}
	if len(envelope.Body) == 0 && envelope.StatusCodeValue.String() == "" {
		return qoderEnvelope{}, false
	}
	return envelope, true
}

// status reports the code the envelope carries. A frame without one is success,
// because the vendor's error frames always state it.
func (e qoderEnvelope) status() int {
	value, err := e.StatusCodeValue.Int64()
	if err != nil || value == 0 {
		return http.StatusOK
	}
	return int(value)
}

// innerPayload returns the OpenAI chunk the envelope carries. A body of "[DONE]" is
// the terminal marker and an empty body is a frame with nothing in it, so both report
// no payload.
func (e qoderEnvelope) innerPayload() ([]byte, bool) {
	trimmed := bytes.TrimSpace(e.Body)
	if len(trimmed) == 0 {
		return nil, false
	}
	var asString string
	if err := json.Unmarshal(trimmed, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" || asString == "[DONE]" {
			return nil, false
		}
		return []byte(asString), true
	}
	return trimmed, true
}

// qoderStreamFailure renders a refusal as the failure the router can act on. The
// vendor's `body` is a JSON string of its own, so its code and message are read out
// of that rather than reported as an opaque blob.
func qoderStreamFailure(envelope qoderEnvelope) *StreamFailure {
	inner, ok := envelope.innerPayload()
	if !ok {
		inner = bytes.TrimSpace(envelope.Body)
	}
	message := string(inner)
	if message == "" || message == "null" {
		message = fmt.Sprintf("the provider returned http %d", envelope.status())
	}
	// Code and details are read as raw members: the vendor sends `code` as a number
	// (112), as a numeric string ("112"), and as a word ("provider_error") in
	// different refusals, and a struct that can only hold one of those loses the
	// whole answer rather than the one member.
	var payload struct {
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	}
	if json.Unmarshal(inner, &payload) == nil {
		if payload.Message != "" {
			message = payload.Message
		}
		quota := isQoderBillingCode(qoderFieldText(payload.Code)) || strings.Contains(message, "pricingUrl")
		// The model backend's own reason rides in `details` as a second JSON string.
		// It is the sentence an operator can act on: "Error in upstream response"
		// names nothing, while "Workspace allocated quota exceeded" names the limit
		// that was reached — and the vendor wraps that behind a generic 429 rather
		// than the numeric billing codes.
		if reason, nested := qoderQuotaDetail(qoderFieldText(payload.Details)); nested && reason != "" {
			// The vendor's own sentence explains a refusal its status does not
			// name — and the pool it complains about recovers: measured 2026-09-28,
			// the identical request was refused twice and served on the third try.
			// So this takes over the message without claiming the account is spent.
			// Marking it quota would park the key and cancel the very retry that
			// gets the answer.
			message = reason
		}
		if quota {
			return &StreamFailure{Status: http.StatusForbidden, Message: message, Quota: true}
		}
	}
	if status := envelope.status(); status >= http.StatusBadRequest && status <= 599 {
		return &StreamFailure{Status: status, Message: message}
	}
	return &StreamFailure{Status: http.StatusBadGateway, Message: message}
}

// qoderFieldText reads one envelope member as text whether the vendor sent it quoted
// or bare, which is the difference between seeing `provider_error` and losing the
// frame's whole payload to a type error.
func qoderFieldText(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var asString string
	if json.Unmarshal([]byte(trimmed), &asString) == nil {
		return asString
	}
	return trimmed
}

// qoderQuotaDetail reads the capacity refusal the vendor nests in `details`. The
// model backend answers a spent allocation as `insufficient_quota` under a generic
// provider_error envelope, which is a spent account wearing a throttle's status.
func qoderQuotaDetail(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	var wrapped struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(trimmed), &wrapped) != nil {
		return "", false
	}
	if wrapped.Error.Code != "insufficient_quota" && wrapped.Error.Type != "insufficient_quota" {
		return "", false
	}
	return strings.TrimSpace(wrapped.Error.Message), true
}

// isQoderBillingCode recognises the codes the vendor sends when an account is spent
// rather than transiently failing: 110 daily limit, 112 quota, 10605 queue throttle.
func isQoderBillingCode(code string) bool {
	return code == "110" || code == "112" || code == "10605"
}

// qoderErrorFrame closes a stream with the provider's reason as visible content and
// the answer's own finish — the same shape the reference emits when a refusal arrives
// too late to change the status the client already has.
func qoderErrorFrame(envelope qoderEnvelope) []byte {
	message := strings.TrimSpace(string(envelope.Body))
	if message == "" {
		message = fmt.Sprintf("qoder error %d", envelope.status())
	}
	if len(message) > 200 {
		message = message[:200]
	}
	now := time.Now()
	chunk := qoderErrorChunk{
		ID:      fmt.Sprintf("qoder-error-%d", now.UnixMilli()),
		Object:  "chat.completion.chunk",
		Created: now.Unix(),
		Choices: []qoderErrorChoice{{
			Index:        0,
			Delta:        qoderErrorDelta{Content: fmt.Sprintf("\n[qoder error %d: %s]", envelope.status(), message)},
			FinishReason: "stop",
		}},
	}
	raw, err := json.Marshal(chunk)
	if err != nil {
		return []byte("data: [DONE]\n\n")
	}
	return append(appendSSEFrame(raw), []byte("data: [DONE]\n\n")...)
}

// appendSSEFrame renders one payload as an SSE data event.
func appendSSEFrame(payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+9)
	frame = append(frame, "data: "...)
	frame = append(frame, bytes.TrimSpace(payload)...)
	return append(frame, '\n', '\n')
}
