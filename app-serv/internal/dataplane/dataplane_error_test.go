// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/dataplane_error_test.go
// @for       Table-driven tests for the data-plane error codes and their status mapping.
// @uses      errors, testing, internal/domain
// @reason    SPEC-API-001 §4 and §8 fix the OpenAI error envelope and the HTTP status each code
//
//	answers with: MODEL_NOT_FOUND and PROVIDER_NOT_ROUTABLE are client errors, and an
//	upstream failure is a 502. A code that maps to the wrong status is what turns an
//	actionable refusal into an outage-shaped one, so both mappings are pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestDataPlaneErrorStatus pins the OpenAI envelope mapping (SPEC-API-001 §4, §8):
// MODEL_NOT_FOUND and PROVIDER_NOT_ROUTABLE are client errors, and an upstream
// failure keeps its own class.
func TestDataPlaneErrorStatus(t *testing.T) {
	cases := []struct {
		name       string
		code       string
		wantStatus int
		wantType   string
	}{
		{name: "validation", code: CodeValidation, wantStatus: 400, wantType: "invalid_request_error"},
		{name: "model not found", code: CodeModelNotFound, wantStatus: 400, wantType: "invalid_request_error"},
		{name: "provider not routable", code: CodeProviderNotRoutable, wantStatus: 400, wantType: "invalid_request_error"},
		{name: "unauthorized", code: CodeUnauthorized, wantStatus: 401, wantType: "authentication_error"},
		{name: "rate limited", code: CodeRateLimited, wantStatus: 429, wantType: "rate_limit_error"},
		{name: "no provider", code: CodeNoProvider, wantStatus: 503, wantType: "server_error"},
		{name: "upstream error", code: CodeUpstreamError, wantStatus: 502, wantType: "server_error"},
		{name: "upstream timeout", code: CodeUpstreamTimeout, wantStatus: 504, wantType: "server_error"},
		{name: "internal", code: CodeInternal, wantStatus: 500, wantType: "server_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failure := dataPlaneError(tc.code, "a message")
			if failure.OpenAIStatus() != tc.wantStatus {
				t.Fatalf("status = %d, want %d", failure.OpenAIStatus(), tc.wantStatus)
			}
			if failure.Type != tc.wantType {
				t.Fatalf("type = %q, want %q", failure.Type, tc.wantType)
			}
			if failure.Message != "a message" {
				t.Fatalf("message = %q, want the English message passed in", failure.Message)
			}
		})
	}
}

// TestAsError_ManagementMapping pins that a management error from a shared
// dependency does not leak a management code to a CLI tool.
func TestAsError_ManagementMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{name: "a not-found maps to MODEL_NOT_FOUND", err: domain.NewNotFoundError("gone"), wantCode: CodeModelNotFound},
		{name: "an unauthorized maps to UNAUTHORIZED", err: domain.NewUnauthorizedError("no"), wantCode: CodeUnauthorized},
		{name: "a conflict maps to INTERNAL_ERROR", err: domain.NewConflictError("dup"), wantCode: CodeInternal},
		{name: "a plain error maps to INTERNAL_ERROR", err: errors.New("boom"), wantCode: CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AsError(tc.err).Code; got != tc.wantCode {
				t.Fatalf("code = %q, want %q", got, tc.wantCode)
			}
		})
	}
	if got := AsError(nil); got != nil {
		t.Fatalf("AsError(nil) = %+v, want nil", got)
	}
}
