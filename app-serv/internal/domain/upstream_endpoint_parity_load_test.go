// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_parity_load_test.go
// @for       The load path for the connection-parity fields.
// @uses      testing, time.
// @reason    Draft 017 §4.1b adds five fields, and the load path is where they are
//
//	easiest to lose: a rehydrate that forgets one reports the zero value on
//	every read while every write looks correct. Keeping it apart from the
//	mutation tests keeps both files inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-23
package domain

import (
	"testing"
)

// TestRehydrateUpstreamEndpoint_CarriesTheParityFields pins the load path: a
// stored row's parity fields must survive a round trip, or every read would
// silently report the zero value.
func TestRehydrateUpstreamEndpoint_CarriesTheParityFields(t *testing.T) {
	at := parityNow
	endpoint := RehydrateUpstreamEndpoint(
		"ep_01TEST", "openai", "Primary", UpstreamAuthAPIKey, 1, UpstreamEndpointActive,
		nil, EndpointAccount{}, EndpointTestStatus{},
		nil, nil, parityNow, parityNow, nil,
		EndpointParity{
			GlobalPriority:      5,
			DefaultModel:        "gpt-4o",
			ConsecutiveUseCount: 3,
			LastErrorCode:       "UPSTREAM_ERROR",
			LastErrorMessage:    "the upstream answered 500",
			LastErrorAt:         &at,
			ProxyPoolID:         "proxy-01ABC",
		},
	)
	if endpoint.GlobalPriority() != 5 {
		t.Fatalf("GlobalPriority() = %d, want 5", endpoint.GlobalPriority())
	}
	if endpoint.DefaultModel() != "gpt-4o" {
		t.Fatalf("DefaultModel() = %q, want gpt-4o", endpoint.DefaultModel())
	}
	if endpoint.ConsecutiveUseCount() != 3 {
		t.Fatalf("ConsecutiveUseCount() = %d, want 3", endpoint.ConsecutiveUseCount())
	}
	if endpoint.ProxyPoolID() != "proxy-01ABC" {
		t.Fatalf("ProxyPoolID() = %q, want proxy-01ABC", endpoint.ProxyPoolID())
	}
	code, message, errAt := endpoint.LastError()
	if code != "UPSTREAM_ERROR" || message != "the upstream answered 500" || errAt == nil {
		t.Fatalf("LastError() = (%q, %q, %v), want the stored triple", code, message, errAt)
	}
}
