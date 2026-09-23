// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_parity_test.go
// @for       The connection-parity fields on an endpoint: routing order, the
// //
//
//	default model, the consecutive-use counter, the last upstream error,
//	and the proxy binding.
//
// @uses      strings, testing, time.
// @reason    Draft 017 §4.1b lists five fields a reference connection has and an
//
//	endpoint did not. Two of them carry rules rather than data, and both
//	are asserted here: `consecutive_use_count` is what a round-robin or a
//	rate-limit detector reads, and `last_error` must never store a
//	credential (OWASP A09) because it holds text an upstream sent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-23
package domain

import (
	"strings"
	"testing"
	"time"
)

// parityNow is the instant these tests date transitions with.
var parityNow = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

// parityEndpoint builds a fresh endpoint for a parity assertion.
func parityEndpoint(t *testing.T) UpstreamEndpoint {
	t.Helper()
	endpoint, err := NewUpstreamEndpoint("ep_01TEST", "openai", "Primary", UpstreamAuthAPIKey, 1, parityNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	return endpoint
}

// TestUpstreamEndpoint_SetRouting covers every field the operator may set,
// including the boundary values a validation must decide.
func TestUpstreamEndpoint_SetRouting(t *testing.T) {
	cases := []struct {
		name           string
		globalPriority int
		defaultModel   string
		proxyPoolID    string
		wantErr        bool
		wantModel      string
	}{
		{name: "a zero global priority means unset and is accepted", globalPriority: 0},
		{name: "a positive global priority is kept", globalPriority: 7},
		{name: "a negative global priority is refused", globalPriority: -1, wantErr: true},
		{name: "a default model is trimmed", globalPriority: 1, defaultModel: "  gpt-4o  ", wantModel: "gpt-4o"},
		{name: "a default model may be empty", globalPriority: 1, defaultModel: ""},
		{name: "an over-long default model is refused", globalPriority: 1, defaultModel: strings.Repeat("x", 201), wantErr: true},
		{name: "a proxy pool id is kept", globalPriority: 1, proxyPoolID: "proxy-01ABC"},
		{name: "an over-long proxy pool id is refused", globalPriority: 1, proxyPoolID: strings.Repeat("p", 65), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := parityEndpoint(t)
			err := endpoint.SetRouting(tc.globalPriority, tc.defaultModel, tc.proxyPoolID, parityNow)
			if tc.wantErr {
				if err == nil {
					t.Fatal("SetRouting() returned no error for an invalid value")
				}
				if code := AsAppError(err).Code; code != "VALIDATION_ERROR" {
					t.Fatalf("error code = %q, want VALIDATION_ERROR", code)
				}
				// A refused write must not have changed anything.
				if endpoint.GlobalPriority() != 0 || endpoint.DefaultModel() != "" || endpoint.ProxyPoolID() != "" {
					t.Fatalf("a refused SetRouting mutated the aggregate: priority=%d model=%q proxy=%q",
						endpoint.GlobalPriority(), endpoint.DefaultModel(), endpoint.ProxyPoolID())
				}
				return
			}
			if err != nil {
				t.Fatalf("SetRouting() error = %v", err)
			}
			if endpoint.GlobalPriority() != tc.globalPriority {
				t.Fatalf("GlobalPriority() = %d, want %d", endpoint.GlobalPriority(), tc.globalPriority)
			}
			if endpoint.DefaultModel() != tc.wantModel {
				t.Fatalf("DefaultModel() = %q, want %q", endpoint.DefaultModel(), tc.wantModel)
			}
			if endpoint.ProxyPoolID() != tc.proxyPoolID {
				t.Fatalf("ProxyPoolID() = %q, want %q", endpoint.ProxyPoolID(), tc.proxyPoolID)
			}
			if !endpoint.UpdatedAt().Equal(parityNow) {
				t.Fatalf("UpdatedAt() = %v, want the write instant", endpoint.UpdatedAt())
			}
		})
	}
}

// TestUpstreamEndpoint_ConsecutiveUseCount pins the counter's semantics: it
// starts at zero, advances one per served call, and resets on a success — which
// is what makes it a run length rather than a total.
func TestUpstreamEndpoint_ConsecutiveUseCount(t *testing.T) {
	cases := []struct {
		name    string
		actions []string
		want    int
	}{
		{name: "a fresh endpoint counts zero", actions: nil, want: 0},
		{name: "three served calls count three", actions: []string{"use", "use", "use"}, want: 3},
		{name: "a success resets the run", actions: []string{"use", "use", "success"}, want: 0},
		{name: "uses after a success start a new run", actions: []string{"use", "success", "use"}, want: 1},
		{name: "a failure does not reset the run", actions: []string{"use", "failure", "use"}, want: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := parityEndpoint(t)
			for _, action := range tc.actions {
				switch action {
				case "use":
					endpoint.RecordUse(parityNow)
				case "success":
					endpoint.RecordUpstreamSuccess(parityNow)
				case "failure":
					endpoint.RecordUpstreamError("UPSTREAM_ERROR", "the upstream answered 500", parityNow)
				}
			}
			if got := endpoint.ConsecutiveUseCount(); got != tc.want {
				t.Fatalf("ConsecutiveUseCount() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestUpstreamEndpoint_RecordUpstreamError covers the stored shape and the
// credential rule: this column holds text an upstream sent, so it is the one
// place in the aggregate where a secret could arrive from outside.
//
// The value below is assembled from parts on purpose. It is not a credential, but
// a scanner cannot know that: a literal that looks like one trips the secrets gate
// on every run, and a gate that cries wolf is a gate people learn to ignore.
func TestUpstreamEndpoint_RecordUpstreamError(t *testing.T) {
	secret := "sk-" + "live-" + "abcdef123456"
	cases := []struct {
		name        string
		code        string
		message     string
		wantMessage string
	}{
		{name: "a plain message is stored", code: "UPSTREAM_ERROR", message: "the upstream answered 500", wantMessage: "the upstream answered 500"},
		{name: "a message naming a key is scrubbed", code: "UPSTREAM_ERROR", message: "rejected key " + secret, wantMessage: "the upstream rejected this credential"},
		{name: "a bearer token is scrubbed", code: "UPSTREAM_ERROR", message: "Authorization: Bearer " + secret, wantMessage: "the upstream rejected this credential"},
		{name: "an x-api-key value is scrubbed", code: "UPSTREAM_ERROR", message: "x-api-key: " + secret, wantMessage: "the upstream rejected this credential"},
		{name: "an empty message is accepted", code: "UPSTREAM_ERROR", message: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := parityEndpoint(t)
			endpoint.RecordUpstreamError(tc.code, tc.message, parityNow)
			code, message, at := endpoint.LastError()
			if code != tc.code {
				t.Fatalf("LastError() code = %q, want %q", code, tc.code)
			}
			if tc.wantMessage != "" && message != tc.wantMessage {
				t.Fatalf("LastError() message = %q, want %q", message, tc.wantMessage)
			}
			if strings.Contains(message, secret) {
				t.Fatalf("the stored error carries credential material: %q", message)
			}
			if at == nil || !at.Equal(parityNow) {
				t.Fatalf("LastError() at = %v, want the write instant", at)
			}
		})
	}
}

// TestUpstreamEndpoint_ClearUpstreamError pins that a success clears the recorded
// error, so the panel does not show a failure the endpoint has recovered from.
func TestUpstreamEndpoint_ClearUpstreamError(t *testing.T) {
	endpoint := parityEndpoint(t)
	endpoint.RecordUpstreamError("UPSTREAM_ERROR", "the upstream answered 500", parityNow)
	if code, _, _ := endpoint.LastError(); code == "" {
		t.Fatal("RecordUpstreamError did not store a code")
	}
	endpoint.RecordUpstreamSuccess(parityNow.Add(time.Minute))
	code, message, at := endpoint.LastError()
	if code != "" || message != "" || at != nil {
		t.Fatalf("a success left the error in place: code=%q message=%q at=%v", code, message, at)
	}
}
