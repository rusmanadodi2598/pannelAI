// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_multiaccount_test.go
// @for       Several identity-less OAuth accounts of one provider, each its own row.
// @uses      context, fmt, testing, time, internal/registry.
// @reason    A vendor that returns no user identity gives the gateway nothing to dedup on,
//
//	and the reference's answer is to insert a new connection each time
//	(connectionsRepo.js:133 only dedups when an email exists). Our state round used to
//	manufacture a constant synthetic email instead, which made every login after the
//	first overwrite the stored credential. These tests hold the multi-account shape the
//	panel already renders: distinct rows, numbered labels, no invented identity, and a
//	first account whose token survives the second login.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package service

import (
	"context"
	"fmt"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// scriptStateRounds hands each login its own vendor round and its own token, so a
// test can tell the stored rows apart by the credential they hold.
func scriptStateRounds(fixture *oauthFlowFixture) {
	fixture.tokens.stateRoundFn = func(*registry.OAuth) (oauthhttp.StateRound, error) {
		// The fake counts the call before handing it over, so the counter is this round.
		n := fixture.tokens.stateRoundCalls
		return oauthhttp.StateRound{
			State:    fmt.Sprintf("state-%d", n),
			AuthURL:  fmt.Sprintf("https://vendor.example.com/login?state=state-%d", n),
			Interval: 5 * time.Second, Expires: oauthhttp.StateRoundRetryWindow,
		}, nil
	}
	fixture.tokens.statePollFn = func(state string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{
			AccessToken:  "access-" + state,
			RefreshToken: "refresh-" + state,
			ExpiresAt:    testNow.Add(2 * time.Hour),
		}, false, nil
	}
}

// connectOneRound runs one full state login and returns the endpoint it stored.
func connectOneRound(t *testing.T, fixture oauthFlowFixture) string {
	t.Helper()
	code := startDevice(t, fixture, stateProvider).DeviceCode
	outcome, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{
		ProviderID: stateProvider, DeviceCode: code,
	})
	if err != nil {
		t.Fatalf("DevicePoll() error = %v", err)
	}
	if outcome.Status != "connected" || outcome.EndpointID == "" {
		t.Fatalf("poll outcome = %+v, want a connected endpoint", outcome)
	}
	if !outcome.Created {
		t.Fatalf("poll reported Created=false for %s: an identity-less login matched an account it cannot identify", outcome.EndpointID)
	}
	return outcome.EndpointID
}

func openedToken(t *testing.T, fixture oauthFlowFixture, endpointID string) string {
	t.Helper()
	endpoint, err := fixture.store.GetByID(context.Background(), endpointID)
	if err != nil {
		t.Fatalf("GetByID(%q) error = %v", endpointID, err)
	}
	credential := endpoint.OAuth()
	if credential == nil {
		t.Fatalf("endpoint %s has no credential", endpointID)
	}
	value, err := fixture.sealer.Open(credential.AccessTokenEncrypted())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return value
}

// TestStateRound_EachLoginStoresItsOwnAccount pins the multi-account shape: three
// approvals of the same region are three accounts, not one account rewritten three
// times, and the first one keeps the credential it was given.
func TestStateRound_EachLoginStoresItsOwnAccount(t *testing.T) {
	fixture := newStateFlowFixture(t)
	scriptStateRounds(&fixture)

	ids := []string{connectOneRound(t, fixture), connectOneRound(t, fixture), connectOneRound(t, fixture)}
	seen := map[string]bool{}
	for i, id := range ids {
		if seen[id] {
			t.Fatalf("login %d reused endpoint %s: the earlier account's token is gone", i+1, id)
		}
		seen[id] = true
		if got := openedToken(t, fixture, id); got != fmt.Sprintf("access-state-%d", i+1) {
			t.Fatalf("endpoint %s stores %q, want the credential its own round was granted", id, got)
		}
	}
}

// TestStateRound_AccountsAreNumberedPerProvider pins the label an identity-less
// account gets. The upstream store enforces UNIQUE (provider_id, label)
// (migrations/000005), so a constant fallback label would refuse the second row at
// insert; the reference numbers the same way ("Account N", connectionsRepo.js:181).
func TestStateRound_AccountsAreNumberedPerProvider(t *testing.T) {
	fixture := newStateFlowFixture(t)
	scriptStateRounds(&fixture)

	ids := []string{connectOneRound(t, fixture), connectOneRound(t, fixture), connectOneRound(t, fixture)}
	for i, id := range ids {
		endpoint, err := fixture.store.GetByID(context.Background(), id)
		if err != nil {
			t.Fatalf("GetByID(%q) error = %v", id, err)
		}
		want := fmt.Sprintf("Account %d", i+1)
		if endpoint.Label() != want {
			t.Fatalf("label = %q, want %q", endpoint.Label(), want)
		}
	}
}

// TestStateRound_AccountsCarryNoInventedIdentity states the rule the dedup depends
// on: with nothing from the vendor, the account names no email, no display name and
// no workspace, so `FindOAuthEndpoint` matches no branch and cannot spend a lookup
// that merges two people's accounts. The machine id each round generated is kept,
// because it is genuinely the round's own.
func TestStateRound_AccountsCarryNoInventedIdentity(t *testing.T) {
	fixture := newStateFlowFixture(t)
	scriptStateRounds(&fixture)

	first := connectOneRound(t, fixture)
	second := connectOneRound(t, fixture)

	accountOne := mustAccountOf(t, fixture, first)
	if accountOne.Email().String() != "" || accountOne.Name() != "" || accountOne.WorkspaceID() != "" {
		t.Fatalf("account = %+v, want no identity fields: this vendor returns none", accountOne)
	}
	if accountOne.MachineID() == "" {
		t.Fatal("account carries no machine id, so the round that made it cannot be traced")
	}
	if mustAccountOf(t, fixture, second).MachineID() == accountOne.MachineID() {
		t.Fatal("two rounds stored the same machine id: the second account is a copy, not a login")
	}
}

func mustAccountOf(t *testing.T, fixture oauthFlowFixture, endpointID string) domain.EndpointAccount {
	t.Helper()
	endpoint, err := fixture.store.GetByID(context.Background(), endpointID)
	if err != nil {
		t.Fatalf("GetByID(%q) error = %v", endpointID, err)
	}
	return endpoint.Account()
}

// TestStateRound_NumberedLabelTakesAFreeName proves the label is chosen against the
// names already in use rather than against a row count: with `Account 1` deleted and
// `Account 2` still standing, a count would hand the new login a name the survivor
// holds, and UNIQUE (provider_id, label) would refuse the account the vendor granted.
func TestStateRound_NumberedLabelTakesAFreeName(t *testing.T) {
	fixture := newStateFlowFixture(t)
	scriptStateRounds(&fixture)

	seed, err := domain.NewUpstreamEndpoint("ep-seed-account", stateProvider, "Account 2",
		domain.UpstreamAuthOAuth, 1, testNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	if err := fixture.store.Create(context.Background(), seed); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}

	endpointID := connectOneRound(t, fixture)
	endpoint, err := fixture.store.GetByID(context.Background(), endpointID)
	if err != nil {
		t.Fatalf("GetByID(%q) error = %v", endpointID, err)
	}
	if endpoint.Label() != "Account 1" {
		t.Fatalf("label = %q, want the first free name, not the one the survivor already holds", endpoint.Label())
	}
}
