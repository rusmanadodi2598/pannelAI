// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_callback_reflection_test.go
// @for       The state-before-error rule on an OAuth callback that carries provider text.
// @uses      context, strings, testing.
// @reason    The callback's redirect carries the provider's error sentence, so an unauthenticated request must not reach it without a staged state, and the sentence must stay bounded once it does.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestOAuthCallbackRequiresStateBeforeAProviderError(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	_, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "identity-provider", Error: "access_denied", ErrorDescription: "the user declined",
	})
	mustAppError(t, err, "VALIDATION_ERROR")
}

func TestOAuthCallbackBoundsAProviderError(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	state := startFlow(t, fixture, "identity-provider")
	_, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "identity-provider", State: state,
		Error: "access_denied", ErrorDescription: strings.Repeat("x", maxOAuthProviderError*2),
	})
	mustAppError(t, err, "UPSTREAM_ERROR")
	if got := domain.AsAppError(err).Message; len([]rune(got)) > maxOAuthProviderError+len("the provider refused the authorization: access_denied: ") {
		t.Fatalf("message length = %d, want the provider text bounded", len([]rune(got)))
	}
}
