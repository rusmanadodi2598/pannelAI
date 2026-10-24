// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_billing_failure_test.go
// @for       Tests pinning that an upstream 402 is classified as billing rather than as a dead credential, and what that changes for the chain, the client, and the panel.
// @uses      context, net/http, testing, internal/domain.
// @reason    Draft 050 D2: 402 shared the credential basket, so an account with no vendor credit was parked for two minutes and reported as UPSTREAM_ERROR, which told the client the gateway had an outage and told the operator nothing about the remedy. The class is separate now, with a ladder measured in hours and a code of its own; all three are pinned here because each is a different surface of the same decision.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-13
package dataplane

import (
	"context"
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestFailureClass_SplitsBillingFromTheCredential pins the classification itself:
// 401 and 403 are about the credential, 402 is about the account's credit, and the
// three must not share a window.
func TestFailureClass_SplitsBillingFromTheCredential(t *testing.T) {
	cases := []struct {
		status int
		want   domain.KeyFailureClass
	}{
		{http.StatusUnauthorized, domain.KeyFailureAuth},
		{http.StatusForbidden, domain.KeyFailureAuth},
		{http.StatusPaymentRequired, domain.KeyFailureBilling},
	}
	for _, tc := range cases {
		if got := FailureClass(tc.status); got != tc.want {
			t.Fatalf("FailureClass(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
	if !domain.KeyFailureBilling.ImplicatesCredential() {
		t.Fatal("a billing failure must implicate the credential: the account behind it cannot serve")
	}
	if !failoverWorthy(CodeBillingExhausted) {
		t.Fatal("the chain must keep walking: the next account may have credit")
	}
}

// TestRelay_ExhaustedCreditReportsItsOwnCode pins what the client and the panel see
// when every account answers 402: the named code rather than a generic upstream
// outage, and a key parked on the billing ladder rather than the credential one.
func TestRelay_ExhaustedCreditReportsItsOwnCode(t *testing.T) {
	var calls int
	server := alwaysRefusingUpstream(t, &calls, http.StatusPaymentRequired)
	repo := multiKeyRelayRepo(t)
	engine := newRelayEngine(t, server.URL, repo, nil)

	_, err := engine.Relay(context.Background(), relayRequest("alpha/m"), nil)
	failure := AsError(err)
	if failure == nil {
		t.Fatal("Relay() = nil error, want the exhausted chain's failure")
	}
	if failure.Code != CodeBillingExhausted {
		t.Fatalf("code = %s, want %s: UPSTREAM_ERROR would tell the client the gateway broke",
			failure.Code, CodeBillingExhausted)
	}
	if failure.Status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: the client cannot pay the gateway's vendor bill", failure.Status)
	}

	// The window itself is pinned in the domain, where the clock is a fixture; the
	// relay's selector reads the real one, so this level can only say the key left
	// rotation and how the failure was named.
	parked := repo.health["uky-multi-a"]
	if parked.Status() != domain.UpstreamKeyError {
		t.Fatalf("key status = %q, want the account's key parked", parked.Status())
	}
	if parked.RateLimitedUntil() == nil {
		t.Fatal("the key carries no park window")
	}

	parity, seen := repo.outcome["ep-multi"]
	if !seen {
		t.Fatal("endpoint parity was never written, so the panel cannot see the refusal")
	}
	if code, _, _ := parity.LastError(); code != CodeBillingExhausted {
		t.Fatalf("parity code = %q, want it to name the condition the operator fixes", code)
	}
}

// TestRelay_CredentialRefusalKeepsTheCredentialWindow is the other half of the split:
// a 401 still answers as an upstream failure and still parks the key that carried it,
// so naming 402 did not swallow the basket the credential failures were already in.
func TestRelay_CredentialRefusalKeepsTheCredentialWindow(t *testing.T) {
	var calls int
	server := alwaysRefusingUpstream(t, &calls, http.StatusUnauthorized)
	repo := multiKeyRelayRepo(t)
	engine := newRelayEngine(t, server.URL, repo, nil)

	_, err := engine.Relay(context.Background(), relayRequest("alpha/m"), nil)
	failure := AsError(err)
	if failure.Code == CodeBillingExhausted {
		t.Fatal("a 401 was reported as exhausted credit")
	}
	if failure.Status != http.StatusBadGateway {
		t.Fatalf("status = %d, want the credential refusal to keep answering as an upstream failure", failure.Status)
	}
	parked := repo.health["uky-multi-a"]
	if parked.Status() != domain.UpstreamKeyError || parked.RateLimitedUntil() == nil {
		t.Fatal("a 401 no longer parks the credential that carried it")
	}
	if code, _, _ := repo.outcome["ep-multi"].LastError(); code == CodeBillingExhausted {
		t.Fatal("a credential refusal was recorded as exhausted credit")
	}
}
