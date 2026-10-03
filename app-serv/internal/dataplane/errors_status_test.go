// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/errors_status_test.go
// @for       Table-driven tests for the data plane code to HTTP status and error type mapping.
// @uses      net/http, strings, testing
// @reason    SPEC-API-001 §7.15 and §8 publish one status per data plane code, and the OpenAI
//
//	wire is the contract this plane claims compatibility with, so a code mapped to the wrong
//	status is not a cosmetic drift: a client's retry policy branches on it. MODEL_NOT_FOUND
//	lived untested in statusFor until draft 041, which is how §7.15's 404 and the code's 400
//	could disagree for eleven days without a failing test.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-03
package dataplane

import (
	"net/http"
	"strings"
	"testing"
)

func TestStatusFor(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{code: CodeValidation, want: http.StatusBadRequest},
		// §7.15's resolution order ends here, and the OpenAI wire answers a model it
		// does not know with 404 invalid_request_error. A 400 would tell a client the
		// body was malformed, and a retry of the same model name never succeeds.
		{code: CodeModelNotFound, want: http.StatusNotFound},
		{code: CodeProviderNotRoutable, want: http.StatusBadRequest},
		{code: CodeUpstreamRejected, want: http.StatusBadRequest},
		{code: CodeUnauthorized, want: http.StatusUnauthorized},
		{code: CodeRateLimited, want: http.StatusTooManyRequests},
		{code: CodeNoProvider, want: http.StatusServiceUnavailable},
		{code: CodeUpstreamError, want: http.StatusBadGateway},
		{code: CodeUpstreamTimeout, want: http.StatusGatewayTimeout},
		{code: "SOMETHING_UNEXPECTED", want: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			if got := statusFor(tc.code); got != tc.want {
				t.Fatalf("statusFor(%q) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}

// TestTypeFor pins the OpenAI error type beside the status, because a client that
// classifies by type and retries by status needs both to say the same thing.
func TestTypeFor(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{code: CodeValidation, want: "invalid_request_error"},
		{code: CodeModelNotFound, want: "invalid_request_error"},
		{code: CodeProviderNotRoutable, want: "invalid_request_error"},
		{code: CodeUpstreamRejected, want: "invalid_request_error"},
		{code: CodeUnauthorized, want: "authentication_error"},
		{code: CodeRateLimited, want: "rate_limit_error"},
		{code: CodeUpstreamError, want: "server_error"},
		{code: CodeNoProvider, want: "server_error"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			if got := typeFor(tc.code); got != tc.want {
				t.Fatalf("typeFor(%q) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

// TestDataPlaneError_CarriesItsMappedStatus pins the constructor rather than the two
// helpers: a status computed and then dropped on the way out would still pass every
// test above.
func TestDataPlaneError_CarriesItsMappedStatus(t *testing.T) {
	appErr := dataPlaneError(CodeModelNotFound, "model gone")
	if appErr.Status != http.StatusNotFound {
		t.Fatalf("Status = %d, want 404", appErr.Status)
	}
	if appErr.Type != "invalid_request_error" {
		t.Fatalf("Type = %q, want invalid_request_error", appErr.Type)
	}
	if !strings.Contains(appErr.Message, "gone") {
		t.Fatalf("Message = %q, want it to carry the reason", appErr.Message)
	}
}
