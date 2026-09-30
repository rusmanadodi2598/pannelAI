// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_state_test.go
// @for       The state round as a connect path: what the panel is offered, what
//
//	the round holds, and what a granted poll stores.
//
// @uses      context, strings, testing, time.
// @reason    CodeBuddy was reported to the panel as needing a connector, which
//
//	meant no operator could connect it from the screen at all. These tests
//	pin the other answer: the flow is offered, the handle the panel holds
//	is the vendor's own state, a pending poll leaves the round retryable,
//	a granted poll stores a credential the refresh worker can renew —
//	through the header endpoint rather than the form grant — and a second
//	login updates one account instead of stacking another.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

// stateProvider is the international region under its real registry id, so the
// assertions below name the identity an operator actually types.
const stateProvider = "codebuddy-intl"

func newStateFlowFixture(t *testing.T) oauthFlowFixture {
	t.Helper()
	return newOAuthFlowFixture(t, providerWithStateFlow(stateProvider))
}

func TestOAuthStatus_StateRoundIsOfferedAsDevice(t *testing.T) {
	fixture := newStateFlowFixture(t)

	status, err := fixture.service.Status(context.Background(), stateProvider)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Flow != "device" {
		t.Fatalf("flow = %q, want device: the panel offers a link and polls, which is what this vendor answers", status.Flow)
	}
}

func TestStateDeviceStart_HoldsTheVendorsOwnHandle(t *testing.T) {
	fixture := newStateFlowFixture(t)

	start := startDevice(t, fixture, stateProvider)
	if start.DeviceCode != "state-issued" {
		t.Fatalf("DeviceCode = %q, want the state the vendor minted, not a local nonce", start.DeviceCode)
	}
	if !strings.Contains(start.VerificationURL, "vendor.example.com/login") {
		t.Fatalf("VerificationURL = %q, want the authUrl the vendor returned", start.VerificationURL)
	}
	if start.UserCode != "" {
		t.Fatalf("UserCode = %q, want none: this vendor issues no user code", start.UserCode)
	}
	if fixture.tokens.stateRoundCalls != 1 || fixture.tokens.stateRoundPlatform != "ide" {
		t.Fatalf("state round calls = %d with platform %q, want 1 and the entry's ide",
			fixture.tokens.stateRoundCalls, fixture.tokens.stateRoundPlatform)
	}
	staged := stagedDeviceState(t, fixture, start.DeviceCode)
	if staged.State != "state-issued" || staged.Nonce != "" || staged.CodeVerifier != "" {
		t.Fatalf("staged payload = %+v, want the vendor state alone with no PKCE material", staged)
	}
	if staged.MachineID == "" {
		t.Fatal("the staged round carries no machine id, so a re-login could not find its account")
	}
}

func TestStateDevicePoll_PendingKeepsTheRoundRetryable(t *testing.T) {
	fixture := newStateFlowFixture(t)
	code := startDevice(t, fixture, stateProvider).DeviceCode

	outcome, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{
		ProviderID: stateProvider, DeviceCode: code,
	})
	if err != nil {
		t.Fatalf("DevicePoll() error = %v, want a pending wait reported without error", err)
	}
	if outcome.Status != deviceFlowPending {
		t.Fatalf("status = %q, want %q", outcome.Status, deviceFlowPending)
	}
	if len(fixture.store.byID) != 0 {
		t.Fatalf("endpoints stored = %d, want 0: a pending poll connects nothing", len(fixture.store.byID))
	}
	if _, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{
		ProviderID: stateProvider, DeviceCode: code,
	}); err != nil {
		t.Fatalf("second DevicePoll() error = %v, want the round still open after a wait", err)
	}
}

func TestStateDevicePoll_ConnectsAndNamesTheRegion(t *testing.T) {
	fixture := newStateFlowFixture(t)
	fixture.tokens.statePollFn = func(string) (DeviceTokenResponse, bool, error) {
		return DeviceTokenResponse{AccessToken: "cb-access", RefreshToken: "cb-refresh",
			ExpiresAt: testNow.Add(72 * time.Hour)}, false, nil
	}
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
	if len(fixture.tokens.statePollCall) != 1 || fixture.tokens.statePollCall[0] != code {
		t.Fatalf("polls sent = %v, want the vendor state the start returned", fixture.tokens.statePollCall)
	}

	endpoint, err := fixture.store.GetByID(context.Background(), outcome.EndpointID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	// The state round returns no user identity, so the account states none either: an
	// invented email is a dedup key every account of the region would share, which is
	// how a second login used to spend the first one's credential.
	if email := endpoint.Account().Email; email != "" {
		t.Fatalf("account email = %q, want none: this vendor returns no identity to key on", email)
	}
	if credential := endpoint.OAuth(); credential == nil || credential.ExpiresAt == nil {
		t.Fatalf("credential = %+v, want a stored expiry", credential)
	} else if !credential.ExpiresAt.Equal(testNow.Add(72 * time.Hour)) {
		t.Fatalf("expiry = %v, want the lifetime the vendor stated", credential.ExpiresAt)
	}
}

func TestStateRefresh_RotatesThroughTheHeaderEndpoint(t *testing.T) {
	fixture := newStateFlowFixture(t)
	fixture.tokens.statePollFn = func(string) (DeviceTokenResponse, bool, error) {
		return DeviceTokenResponse{AccessToken: "cb-access", RefreshToken: "cb-refresh",
			ExpiresAt: testNow.Add(time.Hour)}, false, nil
	}
	connected, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{
		ProviderID: stateProvider, DeviceCode: startDevice(t, fixture, stateProvider).DeviceCode,
	})
	if err != nil {
		t.Fatalf("DevicePoll() error = %v", err)
	}

	outcome, err := fixture.service.Refresh(context.Background(), stateProvider, connected.EndpointID)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if outcome.Refreshed != 1 {
		t.Fatalf("Refresh() outcome = %+v, want one account renewed", outcome)
	}
	if len(fixture.tokens.stateRefreshCall) != 1 || fixture.tokens.stateRefreshCall[0] != "cb-refresh" {
		t.Fatalf("state refresh calls = %v, want the stored token sent once", fixture.tokens.stateRefreshCall)
	}
	if len(fixture.tokens.grantCalls) != 0 {
		t.Fatalf("form grants issued = %d, want 0: this vendor's refresh endpoint takes no form body",
			len(fixture.tokens.grantCalls))
	}

	endpoint, err := fixture.store.GetByID(context.Background(), connected.EndpointID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	hint := endpoint.OAuth()
	if hint == nil {
		t.Fatal("the endpoint lost its credential through the refresh")
	}
	opened, err := fixture.sealer.Open(hint.RefreshTokenEncrypted)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if opened != "rt-rotated" {
		t.Fatalf("stored refresh token = %q, want the rotated one the vendor returned", opened)
	}
}

// TestDeviceAccountEmail_LeavesIdentitylessRoundsUnKeyed guards the naming rule
// directly: a round whose vendor stated no identity gets no email at all, so its
// login cannot match another person's account, while the PKCE round keeps the
// prefix stored accounts are already matched on.
func TestDeviceAccountEmail_LeavesIdentitylessRoundsUnKeyed(t *testing.T) {
	state := providerWithStateFlow(stateProvider).OAuth
	if got := deviceAccountEmail(state, ""); got != "" {
		t.Fatalf("state email = %q, want none: dedup needs an identity the vendor stated", got)
	}
	qoder := providerWithQoderDeviceFlow("qoder").OAuth
	if got := deviceAccountEmail(qoder, "user-7"); got != "qoder-user-user-7" {
		t.Fatalf("PKCE email = %q, want the historical prefix unchanged: stored accounts match on it", got)
	}
}
