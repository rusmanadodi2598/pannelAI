// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_client_scalars_test.go
// @for       Table-driven tests for the scalar members of a translated request.
// @uses      testing.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, and the scalars a
//
//	client sets are what shape the call: the output ceiling under its
//	Responses spelling, the sampling controls, and the model id. Each
//	is a place a translation can silently drop a client's intent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "testing"

// TestResponsesToOpenAI_CeilingAndStream pins the two scalar rules: the Responses
// ceiling spelling maps onto the chat one, and the caller's stream decision
// reaches the translated body unchanged.
func TestResponsesToOpenAI_CeilingAndStream(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		stream     bool
		wantMax    int
		wantNoCeil bool
	}{
		{
			name:    "max_output_tokens becomes max_tokens",
			body:    `{"model":"m","input":"hi","max_output_tokens":512}`,
			wantMax: 512,
		},
		{
			name:       "an absent ceiling stays absent",
			body:       `{"model":"m","input":"hi"}`,
			wantNoCeil: true,
		},
		{
			name:       "a zero ceiling is refused by validation rather than sent",
			body:       `{"model":"m","input":"hi","max_output_tokens":0}`,
			wantNoCeil: true,
		},
		{
			name:    "a streamed request carries the stream flag",
			body:    `{"model":"m","input":"hi","stream":true,"max_output_tokens":64}`,
			stream:  true,
			wantMax: 64,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := responsesRequest(t, tc.body)
			got := ResponsesToOpenAI(req, "m", tc.stream)
			if got.Stream != tc.stream {
				t.Fatalf("stream = %v, want %v", got.Stream, tc.stream)
			}
			if tc.wantNoCeil {
				if got.MaxTokens != nil {
					t.Fatalf("max tokens = %d, want none", *got.MaxTokens)
				}
				return
			}
			if got.MaxTokens == nil || *got.MaxTokens != tc.wantMax {
				t.Fatalf("max tokens = %v, want %d", got.MaxTokens, tc.wantMax)
			}
		})
	}
}

// TestResponsesToOpenAI_TemperatureAndTopP pins that the sampling controls are
// carried through rather than dropped, since a client that set them expects them
// to reach the model.
func TestResponsesToOpenAI_TemperatureAndTopP(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantTemp *float64
		wantTopP *float64
	}{
		{name: "both are carried through", body: `{"model":"m","input":"hi","temperature":0.5,"top_p":0.9}`, wantTemp: floatPointer(0.5), wantTopP: floatPointer(0.9)},
		{name: "neither is invented when absent", body: `{"model":"m","input":"hi"}`},
		{name: "a zero temperature is kept rather than treated as absent", body: `{"model":"m","input":"hi","temperature":0}`, wantTemp: floatPointer(0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			if (got.Temperature == nil) != (tc.wantTemp == nil) {
				t.Fatalf("temperature = %v, want %v", got.Temperature, tc.wantTemp)
			}
			if tc.wantTemp != nil && *got.Temperature != *tc.wantTemp {
				t.Fatalf("temperature = %v, want %v", *got.Temperature, *tc.wantTemp)
			}
			if (got.TopP == nil) != (tc.wantTopP == nil) {
				t.Fatalf("top_p = %v, want %v", got.TopP, tc.wantTopP)
			}
			if tc.wantTopP != nil && *got.TopP != *tc.wantTopP {
				t.Fatalf("top_p = %v, want %v", *got.TopP, *tc.wantTopP)
			}
		})
	}
}

// TestResponsesToOpenAI_Model pins that the resolved upstream id is what the
// translated body names, not the client's own model string.
func TestResponsesToOpenAI_Model(t *testing.T) {
	got := ResponsesToOpenAI(responsesRequest(t, `{"model":"client/model","input":"hi"}`), "upstream-id", false)
	if got.Model != "upstream-id" {
		t.Fatalf("model = %q, want the resolved upstream id", got.Model)
	}
}

// floatPointer gives a table case a distinct value for "declared zero", which a
// nil pointer has to stay distinguishable from.
func floatPointer(value float64) *float64 { return &value }
