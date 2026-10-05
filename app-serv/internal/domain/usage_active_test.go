// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_active_test.go
// @for       Table-driven tests for the in-flight marker: its invariants, its
//
//	wire codec, and the staleness cutoff the live read is bounded by.
//
// @uses      encoding/json, strings, testing, time.
// @reason    The marker is the one value the live drawing's "routing now" claim
//
//	rests on (R-36), and it crosses a shared Redis instance, so it is
//	both a validated aggregate input and an untrusted decode. Each rule
//	is pinned here: a marker missing its provider would light the wrong
//	node, and a decoder that accepted a foreign member would put a node
//	on screen for a value nothing wrote.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-22
package domain

import (
	"strings"
	"testing"
	"time"
)

// activeRequestFixture is a valid marker, so each case can name one field as the
// variable under test rather than restating the whole value.
//
// It carries no combo on purpose: a call that addressed a single model is the
// dominant traffic, and the codec test pins this fixture's payload as the shape
// an older build already writes.
func activeRequestFixture() ActiveRequest {
	return ActiveRequest{
		MarkerID:   "mark_00000000000000000000000001",
		RequestID:  "req_active_fixture",
		ProviderID: "openai",
		EndpointID: "ep_active_fixture",
		Model:      "gpt-4o",
		StartedAt:  time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

// comboRequestFixture is the same marker for a call that addressed a model combo.
// The combo name travels beside the member model rather than replacing it,
// because the drawing lights both: the combo the client named and the provider
// that answered it.
func comboRequestFixture() ActiveRequest {
	marker := activeRequestFixture()
	marker.Combo = "pro-tier"
	return marker
}

// TestNewActiveRequest covers the constructor's invariants, including the zero
// and boundary values TDD.md §2.5 requires. A marker is only ever built for a
// call that already resolved a provider, so a missing identity is a caller bug
// that must be refused rather than stored.
//
// The input is a struct rather than a positional list because Model and Combo
// are both optional adjacent strings: a transposition compiles, passes
// Validate(), is stored, and draws the wrong label on the panel.
func TestNewActiveRequest(t *testing.T) {
	cases := []struct {
		name      string
		in        ActiveRequestInput
		wantErr   bool
		wantCombo string
	}{
		{name: "a fully identified call is accepted", in: ActiveRequestInput{RequestID: "req_1", ProviderID: "openai", EndpointID: "ep_1", Model: "gpt-4o"}},
		{name: "a call with no endpoint is accepted", in: ActiveRequestInput{RequestID: "req_2", ProviderID: "openai", Model: "gpt-4o"}},
		{name: "a call with no model is accepted", in: ActiveRequestInput{RequestID: "req_3", ProviderID: "openai", EndpointID: "ep_3"}},
		{
			name: "a combo-addressed call keeps its combo", wantCombo: "pro-tier",
			in: ActiveRequestInput{RequestID: "req_5", ProviderID: "openai", EndpointID: "ep_5", Model: "gpt-4o", Combo: "pro-tier"},
		},
		{
			name: "a call with no combo keeps an empty combo",
			in:   ActiveRequestInput{RequestID: "req_6", ProviderID: "openai", Model: "gpt-4o"},
		},
		{name: "an empty request id is refused", in: ActiveRequestInput{ProviderID: "openai", Model: "gpt-4o"}, wantErr: true},
		{name: "an empty provider is refused", in: ActiveRequestInput{RequestID: "req_4", Model: "gpt-4o"}, wantErr: true},
		{
			name:      "a very long identity is accepted",
			wantCombo: strings.Repeat("c", 128),
			in: ActiveRequestInput{
				RequestID: "req_" + strings.Repeat("x", 512), ProviderID: strings.Repeat("p", 128),
				EndpointID: strings.Repeat("e", 128), Model: strings.Repeat("m", 512), Combo: strings.Repeat("c", 128),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
			marker, err := NewActiveRequest(tc.in, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NewActiveRequest() = %+v, want a refusal", marker)
				}
				if marker != (ActiveRequest{}) {
					t.Fatalf("a refused marker = %+v, want the zero value", marker)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewActiveRequest() = %v, want nil", err)
			}
			if marker.MarkerID == "" {
				t.Fatal("the marker carries no id, so two identical calls could not be told apart")
			}
			if marker.Combo != tc.wantCombo {
				t.Fatalf("combo = %q, want %q", marker.Combo, tc.wantCombo)
			}
			if marker.StartedAt != now {
				t.Fatalf("started_at = %v, want %v", marker.StartedAt, now)
			}
			if got := marker.Score(); got != float64(now.UnixMilli()) {
				t.Fatalf("Score() = %v, want %v", got, float64(now.UnixMilli()))
			}
		})
	}
}

// TestActiveRequestValidate covers the read-back invariants: a marker decoded
// from the store is re-validated, so a value nothing in this process wrote is
// refused rather than drawn.
func TestActiveRequestValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ActiveRequest)
		wantErr bool
	}{
		{name: "the fixture is valid", mutate: func(*ActiveRequest) {}},
		{name: "a missing marker id is refused", mutate: func(a *ActiveRequest) { a.MarkerID = "" }, wantErr: true},
		{name: "a missing request id is refused", mutate: func(a *ActiveRequest) { a.RequestID = "" }, wantErr: true},
		{name: "a missing provider is refused", mutate: func(a *ActiveRequest) { a.ProviderID = "" }, wantErr: true},
		{name: "a zero start instant is refused", mutate: func(a *ActiveRequest) { a.StartedAt = time.Time{} }, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker := activeRequestFixture()
			tc.mutate(&marker)
			err := marker.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want a refusal for %+v", marker)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}
