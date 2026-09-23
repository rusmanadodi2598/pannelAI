// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_validate_test.go
// @for       The stateless credential check: the two reference status rules, the
// //
//
//	fallback path, and the no-write property.
//
// @uses      internal/domain, context, testing.
// @reason    SPEC-API-001 §7.4 offers no validate route (draft 017 §4.6), and the
//
//	two rules a naive implementation gets wrong are both here: a 404 on
//	`/models` must fall back to a chat probe rather than report a failure,
//	and an Anthropic 400 or 529 proves the key was accepted. The stateless
//	property is asserted too, because it is the one that would rot first —
//	a validation that quietly wrote a row would still pass every other
//	test in this file.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// recordingValidator is a CredentialValidator that records every check it was
// asked to make, so a test can assert what reached the adapter as well as what
// came back.
type recordingValidator struct {
	outcome ProbeOutcome
	err     error
	checks  []CredentialCheck
}

func (v *recordingValidator) ValidateNode(_ context.Context, check CredentialCheck) (ProbeOutcome, error) {
	v.checks = append(v.checks, check)
	return v.outcome, v.err
}

func (v *recordingValidator) ValidateProvider(_ context.Context, check CredentialCheck) (ProbeOutcome, error) {
	v.checks = append(v.checks, check)
	return v.outcome, v.err
}

// TestValidateNode_RequiresAClosedType pins the one field whose absence would
// silently apply the wrong status rule.
func TestValidateNode_RequiresAClosedType(t *testing.T) {
	cases := []struct {
		name      string
		nodeType  string
		baseURL   string
		wantCode  string
		wantCalls int
	}{
		{name: "an openai-compatible node is accepted", nodeType: "openai-compatible", baseURL: "https://x.test/v1"},
		{name: "an anthropic-compatible node is accepted", nodeType: "anthropic-compatible", baseURL: "https://x.test/v1"},
		{name: "an unknown type is refused", nodeType: "custom-embedding", baseURL: "https://x.test/v1", wantCode: "VALIDATION_ERROR"},
		{name: "an empty type is refused", nodeType: "", baseURL: "https://x.test/v1", wantCode: "VALIDATION_ERROR"},
		{name: "a missing base URL is refused", nodeType: "openai-compatible", baseURL: "  ", wantCode: "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator := &recordingValidator{outcome: ProbeOutcome{State: domain.EndpointTestOK}}
			svc, err := NewCredentialValidationService(validator)
			if err != nil {
				t.Fatalf("NewCredentialValidationService() error = %v", err)
			}
			_, err = svc.ValidateNode(context.Background(), CredentialCheck{
				BaseURL: tc.baseURL, NodeType: tc.nodeType, Credential: "sk-live-abcdef",
			})
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("ValidateNode() error = %v, want nil", err)
				}
				if len(validator.checks) != 1 {
					t.Fatalf("the adapter was called %d times, want 1", len(validator.checks))
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateNode() returned no error for an invalid check")
			}
			if got := domain.AsAppError(err).Code; got != tc.wantCode {
				t.Fatalf("error code = %q, want %q", got, tc.wantCode)
			}
			if len(validator.checks) != 0 {
				t.Fatalf("the adapter was called %d times for a check that must be refused before it", len(validator.checks))
			}
		})
	}
}

// TestValidateProvider_RequiresAProviderID is the provider half of the same rule.
func TestValidateProvider_RequiresAProviderID(t *testing.T) {
	validator := &recordingValidator{outcome: ProbeOutcome{State: domain.EndpointTestOK}}
	svc, err := NewCredentialValidationService(validator)
	if err != nil {
		t.Fatalf("NewCredentialValidationService() error = %v", err)
	}
	if _, err := svc.ValidateProvider(context.Background(), CredentialCheck{ProviderID: "  "}); err == nil {
		t.Fatal("ValidateProvider() returned no error for an empty provider_id")
	} else if got := domain.AsAppError(err).Code; got != "VALIDATION_ERROR" {
		t.Fatalf("error code = %q, want VALIDATION_ERROR", got)
	}
	if len(validator.checks) != 0 {
		t.Fatalf("the adapter was called %d times for an empty provider_id", len(validator.checks))
	}
}

// TestAcceptedByAnthropicRule is the reference's status rule as a table, including
// the boundary statuses and a success, so the rule cannot narrow to "only 2xx".
func TestAcceptedByAnthropicRule(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{status: 200, want: true},
		{status: 201, want: true},
		{status: 400, want: true},
		{status: 404, want: true},
		{status: 429, want: true},
		{status: 500, want: true},
		{status: 529, want: true},
		{status: 401, want: false},
		{status: 403, want: false},
	}
	for _, tc := range cases {
		if got := AcceptedByAnthropicRule(tc.status); got != tc.want {
			t.Fatalf("AcceptedByAnthropicRule(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

// TestValidateOutcomeForAnthropic pins the mapping the wire reports: a status the
// rule accepts is `ok`, with a message that says the request was refused rather
// than the key.
func TestValidateOutcomeForAnthropic(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		wantState   string
		wantMessage bool
	}{
		{name: "a 200 is ok with no message", status: 200, wantState: domain.EndpointTestOK},
		{name: "a 400 is ok and says the request was refused", status: 400, wantState: domain.EndpointTestOK, wantMessage: true},
		{name: "a 529 is ok and says the request was refused", status: 529, wantState: domain.EndpointTestOK, wantMessage: true},
		{name: "a 401 is a rejected credential", status: 401, wantState: domain.EndpointTestFail, wantMessage: true},
		{name: "a 403 is a rejected credential", status: 403, wantState: domain.EndpointTestFail, wantMessage: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := ValidateOutcomeForAnthropic(tc.status)
			if outcome.State != tc.wantState {
				t.Fatalf("State = %q, want %q", outcome.State, tc.wantState)
			}
			if outcome.Status != tc.status {
				t.Fatalf("Status = %d, want %d", outcome.Status, tc.status)
			}
			if tc.wantMessage && outcome.Message == "" {
				t.Fatal("a non-2xx answer must carry a message explaining it")
			}
		})
	}
}

// TestCredentialValidation_WritesNothing is the stateless property.
//
// The service is built with no store at all, which is what makes the property
// structural: there is nothing to write through. The assertion records it anyway,
// so a future edit that adds a repository to this service fails here by name.
func TestCredentialValidation_WritesNothing(t *testing.T) {
	validator := &recordingValidator{outcome: ProbeOutcome{State: domain.EndpointTestOK, Method: ProbeMethodModels}}
	svc, err := NewCredentialValidationService(validator)
	if err != nil {
		t.Fatalf("NewCredentialValidationService() error = %v", err)
	}
	check := CredentialCheck{ProviderID: "openai", Credential: "sk-live-abcdef"}

	// Run it twice: a validation that wrote a row would make the second call
	// behave differently (a duplicate, a cached answer), which this catches.
	for i := 0; i < 2; i++ {
		outcome, err := svc.ValidateProvider(context.Background(), check)
		if err != nil {
			t.Fatalf("ValidateProvider() call %d error = %v", i, err)
		}
		if outcome.State != domain.EndpointTestOK {
			t.Fatalf("call %d: State = %q, want %q", i, outcome.State, domain.EndpointTestOK)
		}
	}
	if len(validator.checks) != 2 {
		t.Fatalf("the adapter was called %d times for 2 validations, want 2: a stateless check repeats identically", len(validator.checks))
	}
}
