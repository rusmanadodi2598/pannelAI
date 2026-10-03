// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_response_parity_test.go
// @for       That an endpoint's connection-parity fields reach the §7.5 wire
//
//	shape the panel renders.
//
// @uses      testing, time, internal/domain, internal/schema.
// @reason    R17 of docs/DRAFT/042-CODE-REVIEW-FIXES.md found the schema's
//
//	parity fields declared but never mapped, so a panel could not show
//	why an endpoint last failed even after the data plane recorded it.
//	The mapper is pinned here so the fields cannot silently drop out of
//	the wire again.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-10-03
package handler

import (
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestToEndpointResponse_CarriesTheParityFields pins the §7.5 mapping: the
// operator-settable routing fields and the run counter render as their own
// members, and the last non-test upstream failure renders as one object that is
// absent while the endpoint has not failed.
func TestToEndpointResponse_CarriesTheParityFields(t *testing.T) {
	endpoint, err := domain.NewUpstreamEndpoint("ep_a", "provider-a", "label-a",
		domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}
	if err := endpoint.SetRouting(7, "gemini-2.5-pro", "pool-1", now); err != nil {
		t.Fatalf("setting routing: %v", err)
	}
	endpoint.RecordUse(now)
	endpoint.RecordUse(now)
	endpoint.RecordUpstreamError("RATE_LIMITED", "the upstream answered 429", now)

	resp := toEndpointResponse(endpoint, now, false)
	if resp.GlobalPriority != 7 {
		t.Fatalf("global_priority = %d, want 7", resp.GlobalPriority)
	}
	if resp.DefaultModel != "gemini-2.5-pro" {
		t.Fatalf("default_model = %q, want gemini-2.5-pro", resp.DefaultModel)
	}
	if resp.ConsecutiveUseCount != 2 {
		t.Fatalf("consecutive_use_count = %d, want 2", resp.ConsecutiveUseCount)
	}
	if resp.ProxyPoolID != "pool-1" {
		t.Fatalf("proxy_pool_id = %q, want pool-1", resp.ProxyPoolID)
	}
	if resp.LastError == nil {
		t.Fatal("last_error = nil, want the recorded failure")
	}
	if resp.LastError.Code != "RATE_LIMITED" {
		t.Fatalf("last_error.code = %q, want RATE_LIMITED", resp.LastError.Code)
	}
	if resp.LastError.Message != "the upstream answered 429" {
		t.Fatalf("last_error.message = %q, want the upstream's own wording", resp.LastError.Message)
	}
	if resp.LastError.At != schema.Timestamp(now) {
		t.Fatalf("last_error.at = %q, want %q", resp.LastError.At, schema.Timestamp(now))
	}
}

// TestToEndpointResponse_OmittedUntilFailed pins that a clean endpoint renders
// no last_error member at all, so a client can distinguish "never failed" from
// "failed with an empty error".
func TestToEndpointResponse_OmittedUntilFailed(t *testing.T) {
	endpoint, err := domain.NewUpstreamEndpoint("ep_a", "provider-a", "label-a",
		domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}

	resp := toEndpointResponse(endpoint, now, false)
	if resp.LastError != nil {
		t.Fatalf("last_error = %+v, want it absent until an upstream failure", resp.LastError)
	}
}

// now is the fixed instant the parity assertions read back.
var now = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
