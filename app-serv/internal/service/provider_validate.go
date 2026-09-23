// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_validate.go
// @for       The stateless credential check: prove a credential before a row
//
//	exists, and say which path proved it.
//
// @uses      internal/domain, context, strings.
// @reason    SPEC-API-001 §7.4 offers no validate route, so the only way to test a
//
//	credential today is to store it first (draft 017 §4.6). The reference
//	validates before the write, and two of its rules are not details: an
//	upstream that does not serve `/models` is still validatable through a
//	one-token chat probe, and for an Anthropic wire a 400 or 529 proves the
//	key was *accepted* — the request was wrong, not the credential.
//
//	The check is stateless by construction: this file holds no store, and
//	the port it declares takes the destination as a value. That is what
//	makes "no row is written" a property rather than a promise.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Probe methods, as the wire reports them. They name which request proved the
// answer, because "valid" from a chat probe and "valid" from a models probe are
// different evidence: the first proves the upstream can serve a request, the
// second only that it can list.
const (
	ProbeMethodModels = "models"
	ProbeMethodChat   = "chat"
)

// CredentialCheck is the request a stateless validation makes: a destination and
// a credential, with nothing stored.
type CredentialCheck struct {
	// BaseURL is the node's base URL, for a node check. Ignored when
	// ProviderID is set.
	BaseURL string
	// NodeType is the node's type ("openai-compatible", "anthropic-compatible").
	NodeType string
	// APIType distinguishes a Responses node from a chat one.
	APIType string
	// ProviderID is a registry provider's id, for a provider check.
	ProviderID string
	// Credential is the plaintext credential. It never leaves this call.
	Credential string
	// ModelID is the model a chat probe names, when the caller supplied one.
	ModelID string
}

// CredentialValidator proves a credential against a destination without storing
// anything.
//
// The two methods are separate rather than one taking a CredentialCheck because
// the destinations differ in kind: a node is a URL the operator typed, a provider
// is an id the registry owns, and the rules that apply differ (a node has no
// declared validate URL; a provider may have one).
type CredentialValidator interface {
	// ValidateNode checks a credential against a node's base URL.
	ValidateNode(ctx context.Context, check CredentialCheck) (ProbeOutcome, error)
	// ValidateProvider checks a credential against a registry provider.
	ValidateProvider(ctx context.Context, check CredentialCheck) (ProbeOutcome, error)
}

// CredentialValidationService implements the two stateless checks.
type CredentialValidationService struct {
	validator CredentialValidator
}

// NewCredentialValidationService validates deps and returns a ready service.
func NewCredentialValidationService(validator CredentialValidator) (*CredentialValidationService, error) {
	if validator == nil {
		return nil, domain.NewValidationError("credential validation requires a validator")
	}
	return &CredentialValidationService{validator: validator}, nil
}

// ValidateNode proves a credential against a node's base URL.
//
// A node's type decides which status rule applies, so it is required and closed:
// an unknown type would fall through to the OpenAI rule and report an Anthropic
// 400 as a rejected key.
func (s *CredentialValidationService) ValidateNode(ctx context.Context, check CredentialCheck) (ProbeOutcome, error) {
	nodeType := strings.TrimSpace(check.NodeType)
	if nodeType != string(domain.NodeOpenAICompatible) && nodeType != string(domain.NodeAnthropicCompatible) {
		return ProbeOutcome{}, domain.NewValidationError("type must be openai-compatible or anthropic-compatible")
	}
	if strings.TrimSpace(check.BaseURL) == "" {
		return ProbeOutcome{}, domain.NewValidationError("base_url is required")
	}
	return s.validator.ValidateNode(ctx, check)
}

// ValidateProvider proves a credential against a registry provider.
func (s *CredentialValidationService) ValidateProvider(ctx context.Context, check CredentialCheck) (ProbeOutcome, error) {
	if strings.TrimSpace(check.ProviderID) == "" {
		return ProbeOutcome{}, domain.NewValidationError("provider_id is required")
	}
	return s.validator.ValidateProvider(ctx, check)
}

// AcceptedByAnthropicRule reports whether an Anthropic-compatible upstream's
// status means the credential was accepted.
//
// The rule is the reference's (providers/validate/route.js:178): only 401 and 403
// mean the key was rejected. A 400 says the *request* was wrong — a bad model, a
// `max_tokens` the model refuses — and a 529 says the provider is overloaded;
// both prove the key was read and accepted, which is the question a validation
// asks. Reporting either as a credential failure sends an operator to replace a
// working key.
func AcceptedByAnthropicRule(status int) bool {
	return status != 401 && status != 403
}

// ValidateOutcomeForAnthropic applies the Anthropic rule to a raw status.
//
// It is exported beside the rule so the adapter that performs the request and the
// test that pins the rule cannot disagree about which statuses mean what.
func ValidateOutcomeForAnthropic(status int) ProbeOutcome {
	outcome := ProbeOutcome{Status: status}
	if status >= 200 && status < 300 {
		outcome.State = domain.EndpointTestOK
		return outcome
	}
	if AcceptedByAnthropicRule(status) {
		outcome.State = domain.EndpointTestOK
		outcome.Message = "the upstream accepted the credential and refused the request"
		return outcome
	}
	outcome.State = domain.EndpointTestFail
	outcome.Message = "the upstream rejected this credential"
	return outcome
}
