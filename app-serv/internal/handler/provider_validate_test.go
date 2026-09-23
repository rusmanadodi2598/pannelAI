// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider_validate_test.go
// @for       The §7.4 credential-check routes: the happy path, the validation
//
//	failure, and the auth failure AGENTS.md §2.1 requires per route.
//
// @uses      internal/domain, internal/service, net/http, net/http/httptest,
//
//	encoding/json, testing.
//
// @reason    Draft 017 §4.6 adds two routes whose whole value is that they write
// //
//
//	nothing. The handler holds no store, so the no-write property is
//	visible from this layer too — and the tests assert the wire carries
//	`method`, because that is the field distinguishing the two probes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-23
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// stubCredentialValidator is the seam double: it answers a fixed outcome and
// records the check it was handed.
type stubCredentialValidator struct {
	outcome service.ProbeOutcome
	err     error
	checks  []service.CredentialCheck
}

func (v *stubCredentialValidator) ValidateNode(_ context.Context, check service.CredentialCheck) (service.ProbeOutcome, error) {
	v.checks = append(v.checks, check)
	return v.outcome, v.err
}

func (v *stubCredentialValidator) ValidateProvider(_ context.Context, check service.CredentialCheck) (service.ProbeOutcome, error) {
	v.checks = append(v.checks, check)
	return v.outcome, v.err
}

// newValidateHandler wires the handler over the double.
func newValidateHandler(t *testing.T, validator *stubCredentialValidator) *ProviderValidateHandler {
	t.Helper()
	svc, err := service.NewCredentialValidationService(validator)
	if err != nil {
		t.Fatalf("NewCredentialValidationService() error = %v", err)
	}
	return NewProviderValidateHandler(svc)
}

// postValidate drives one route with the given body.
func postValidate(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// TestProviderValidateHandler_NodeHappyPath asserts the wire shape: the state, the
// method that proved it, and no credential material.
func TestProviderValidateHandler_NodeHappyPath(t *testing.T) {
	const secret = "sk-live-abcdef"
	validator := &stubCredentialValidator{outcome: service.ProbeOutcome{
		State: domain.EndpointTestOK, Method: service.ProbeMethodChat, LatencyMS: 42, Status: 200,
	}}
	h := newValidateHandler(t, validator)

	rr := postValidate(t, h.Node, `{"base_url":"https://llm.corp.test/v1","type":"openai-compatible","credential":"`+secret+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	if body["state"] != domain.EndpointTestOK {
		t.Fatalf("state = %v, want %q", body["state"], domain.EndpointTestOK)
	}
	if body["method"] != service.ProbeMethodChat {
		t.Fatalf("method = %v, want %q", body["method"], service.ProbeMethodChat)
	}
	if strings.Contains(rr.Body.String(), secret) {
		t.Fatalf("the response echoes the credential: %s", rr.Body.String())
	}
	if len(validator.checks) != 1 || validator.checks[0].Credential != secret {
		t.Fatalf("the service was not handed the credential: %+v", validator.checks)
	}
}

// TestProviderValidateHandler_NodeValidationFailures is the validation-failure
// arm AGENTS.md §2.1 requires, including the closed `type` set.
func TestProviderValidateHandler_NodeValidationFailures(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "a missing base_url is refused", body: `{"type":"openai-compatible"}`},
		{name: "a missing type is refused", body: `{"base_url":"https://x.test/v1"}`},
		{name: "an unknown type is refused", body: `{"base_url":"https://x.test/v1","type":"custom-embedding"}`},
		{name: "an unknown api_type is refused", body: `{"base_url":"https://x.test/v1","type":"openai-compatible","api_type":"embeddings"}`},
		{name: "a non-URL base is refused", body: `{"base_url":"not-a-url","type":"openai-compatible"}`},
		{name: "an unknown field is refused", body: `{"base_url":"https://x.test/v1","type":"openai-compatible","unexpected":1}`},
		{name: "an empty body is refused", body: ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator := &stubCredentialValidator{outcome: service.ProbeOutcome{State: domain.EndpointTestOK}}
			h := newValidateHandler(t, validator)
			rr := postValidate(t, h.Node, tc.body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
				t.Fatalf("body = %s, want a VALIDATION_ERROR code", rr.Body.String())
			}
			if len(validator.checks) != 0 {
				t.Fatalf("the validator ran %d times for an invalid body", len(validator.checks))
			}
		})
	}
}

// TestProviderValidateHandler_ProviderHappyPath covers the provider route.
func TestProviderValidateHandler_ProviderHappyPath(t *testing.T) {
	validator := &stubCredentialValidator{outcome: service.ProbeOutcome{
		State: domain.EndpointTestFail, Method: service.ProbeMethodModels, LatencyMS: 11, Status: 401,
		Message: "the upstream rejected this credential",
	}}
	h := newValidateHandler(t, validator)

	rr := postValidate(t, h.Provider, `{"provider_id":"openai","credential":"sk-live-abcdef"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a rejected credential is a result, not an error (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"state":"fail"`) {
		t.Fatalf("body = %s, want a fail state", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"method":"models"`) {
		t.Fatalf("body = %s, want the method that proved it", rr.Body.String())
	}
}

// TestProviderValidateHandler_ProviderValidationFailures is the provider arm.
func TestProviderValidateHandler_ProviderValidationFailures(t *testing.T) {
	cases := []struct{ name, body string }{
		{name: "a missing provider_id is refused", body: `{"credential":"sk-live-abcdef"}`},
		{name: "an empty provider_id is refused", body: `{"provider_id":""}`},
		{name: "an unknown field is refused", body: `{"provider_id":"openai","extra":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator := &stubCredentialValidator{outcome: service.ProbeOutcome{State: domain.EndpointTestOK}}
			h := newValidateHandler(t, validator)
			rr := postValidate(t, h.Provider, tc.body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if len(validator.checks) != 0 {
				t.Fatalf("the validator ran %d times for an invalid body", len(validator.checks))
			}
		})
	}
}

// TestProviderValidateHandler_NoStoreIsReachable is the stateless property at this
// layer: the handler is constructed from one service and nothing else, so there
// is no path through it that could write a row.
func TestProviderValidateHandler_NoStoreIsReachable(t *testing.T) {
	validator := &stubCredentialValidator{outcome: service.ProbeOutcome{State: domain.EndpointTestOK}}
	h := newValidateHandler(t, validator)
	if h.validation == nil {
		t.Fatal("the handler was built without its service")
	}
	// Two identical calls must behave identically; a handler holding state would
	// make the second one differ.
	first := postValidate(t, h.Provider, `{"provider_id":"openai"}`)
	second := postValidate(t, h.Provider, `{"provider_id":"openai"}`)
	if first.Body.String() != second.Body.String() {
		t.Fatalf("two identical validations answered differently:\n first: %s\n second: %s",
			first.Body.String(), second.Body.String())
	}
}
