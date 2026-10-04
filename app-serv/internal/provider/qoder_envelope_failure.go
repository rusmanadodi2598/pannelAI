// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_envelope_failure.go
// @for       What a Qoder refusal becomes: the failure the router acts on, read
//
//	out of the envelope's own code, message and nested detail.
//
// @uses      bytes, encoding/json, fmt, net/http, strings.
// @reason    A spent account, a throttle and a broken answer all arrive wrapped
//
//	the same way and have to be told apart, because the pool's failover and
//	the account's parking depend on which one this says it is. The frame
//	decoder that produces these values lives beside this file.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-10-04
package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// qoderRawFailureEchoBytes caps how much of a refusal is echoed into the
// client-facing message. The body is already size-bounded upstream, but it is
// the vendor's text — an HTML error page or a debug dump — not a message written
// for this client, and the head of it is enough to name what happened.
const qoderRawFailureEchoBytes = 512

// clampFailureMessage keeps that head, cut on a rune boundary so the client's
// JSON never carries a broken character.
func clampFailureMessage(message string) string {
	if len(message) <= qoderRawFailureEchoBytes {
		return message
	}
	cut := strings.ToValidUTF8(message[:qoderRawFailureEchoBytes], "")
	return strings.TrimRight(cut, "\uFFFD")
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
			return &StreamFailure{Status: http.StatusForbidden, Message: clampFailureMessage(message), Quota: true}
		}
	}
	if status := envelope.status(); status >= http.StatusBadRequest && status <= 599 {
		return &StreamFailure{Status: status, Message: clampFailureMessage(message)}
	}
	return &StreamFailure{Status: http.StatusBadGateway, Message: clampFailureMessage(message)}
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
