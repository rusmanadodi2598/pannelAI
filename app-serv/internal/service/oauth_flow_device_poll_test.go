// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_device_poll_test.go
// @for       One poll of a device round: pending, success, refusal, and what
//
//	the staged state does in each (draft 036 slice A).
//
// @uses      context, errors, strings, testing, time.
// @reason    The poll is the half of the device flow with a state machine
//
//	attached, and the rules worth proving are the quiet ones: a
//	pending answer must not spend the round, a success must spend it
//	exactly once, and a failure must leave it retryable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"errors"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"strings"
	"testing"
	"time"
)

// TestDevicePoll_PendingKeepsTheFlowAlive pins the pending round: the upstream is
// asked with the staged nonce and verifier, and the state survives for the next
// poll.
func TestDevicePoll_PendingKeepsTheFlowAlive(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"))
	start := startDevice(t, fixture, "qoder")
	fixture.tokens.devicePollFn = func(string, string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{}, true, nil
	}

	if answer := poll(t, fixture, "qoder", start.DeviceCode); answer.Status != "pending" {
		t.Fatalf("Status = %q, want pending", answer.Status)
	}
	staged := stagedDeviceState(t, fixture, start.DeviceCode)
	if len(fixture.tokens.devicePollCall) != 2 {
		t.Fatalf("the upstream poll was not attempted")
	}
	if fixture.tokens.devicePollCall[0] != staged.Nonce || fixture.tokens.devicePollCall[1] != staged.CodeVerifier {
		t.Fatalf("poll spent %v, want the staged nonce and verifier", fixture.tokens.devicePollCall)
	}
	if fixture.tokens.infoCalls != 0 {
		t.Fatalf("a pending poll read the account identity: %d calls", fixture.tokens.infoCalls)
	}
}

// TestDevicePoll_SuccessConnectsOnce pins the success round: the upstream token
// lands as a connected endpoint account with the staged machine id, the state is
// consumed exactly once, and a replayed poll is refused.
func TestDevicePoll_SuccessConnectsOnce(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"))
	start := startDevice(t, fixture, "qoder")
	staged := stagedDeviceState(t, fixture, start.DeviceCode)
	expires := testNow.Add(25 * time.Hour)
	fixture.tokens.devicePollFn = func(string, string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{
			AccessToken: "dt-issued", RefreshToken: "rt-issued",
			UserID: "user-7", ExpiresAt: expires,
		}, false, nil
	}
	fixture.tokens.infoFn = func() (oauthhttp.OAuthIdentity, error) {
		return oauthhttp.OAuthIdentity{Name: "Dodi", Email: "dodi@example.com"}, nil
	}

	answer := poll(t, fixture, "qoder", start.DeviceCode)
	if answer.Status != "connected" || answer.EndpointID == "" {
		t.Fatalf("answer = %+v, want a connected endpoint", answer)
	}
	if !answer.Created {
		t.Fatalf("a first connect must create the endpoint")
	}
	if answer.TokenHint == "" || strings.Contains(answer.TokenHint, "dt-issued") {
		t.Fatalf("TokenHint = %q, want a masked hint", answer.TokenHint)
	}

	endpoint, err := fixture.store.GetByID(context.Background(), answer.EndpointID)
	if err != nil {
		t.Fatalf("stored endpoint: %v", err)
	}
	if endpoint.ProviderID() != "qoder" {
		t.Fatalf("endpoint provider = %q", endpoint.ProviderID())
	}
	account := endpoint.Account()
	if account.Email().String() != "dodi@example.com" || account.Name() != "Dodi" || account.WorkspaceID() != "user-7" {
		t.Fatalf("account = %+v, want the userinfo identity with the poll's user id", account)
	}
	if account.MachineID() != staged.MachineID {
		t.Fatalf("the staged machine id did not reach the account")
	}
	credential := endpoint.OAuth()
	if credential == nil || credential.ExpiresAt() == nil || !credential.ExpiresAt().Equal(expires) {
		t.Fatalf("expiry = %v, want the parsed upstream instant %v", credential, expires)
	}

	if _, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{
		ProviderID: "qoder", DeviceCode: start.DeviceCode,
	}); !isValidation(err) {
		t.Fatalf("replayed poll error = %v, want a validation refusal", err)
	}
}

// TestDevicePoll_ReconnectUpdatesTheSameAccount pins the dedup rule a device
// login shares with the callback: the same identity connects once, and a second
// login updates it instead of splitting the account in two.
func TestDevicePoll_ReconnectUpdatesTheSameAccount(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"))
	fixture.tokens.infoFn = func() (oauthhttp.OAuthIdentity, error) {
		return oauthhttp.OAuthIdentity{Name: "Dodi", Email: "dodi@example.com"}, nil
	}
	fixture.tokens.devicePollFn = func(string, string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{AccessToken: "dt-second", UserID: "user-7",
			ExpiresAt: testNow.Add(72 * time.Hour)}, false, nil
	}

	first := poll(t, fixture, "qoder", startDevice(t, fixture, "qoder").DeviceCode)
	second := poll(t, fixture, "qoder", startDevice(t, fixture, "qoder").DeviceCode)

	if second.Created {
		t.Fatalf("a second login for one identity created a new endpoint")
	}
	if second.EndpointID != first.EndpointID {
		t.Fatalf("endpoints differ: %q then %q", first.EndpointID, second.EndpointID)
	}
}

// TestDevicePoll_FailOpenIdentity pins the reference's fail-open userinfo: an
// identity read that fails must not block the connect, and the account then
// falls back to the synthetic email the dedup rule can still match on.
func TestDevicePoll_FailOpenIdentity(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"))
	fixture.tokens.devicePollFn = func(string, string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{AccessToken: "dt-x", UserID: "user-9",
			ExpiresAt: testNow.Add(48 * time.Hour)}, false, nil
	}
	fixture.tokens.infoFn = func() (oauthhttp.OAuthIdentity, error) {
		return oauthhttp.OAuthIdentity{}, errors.New("userinfo down")
	}

	answer := poll(t, fixture, "qoder", startDevice(t, fixture, "qoder").DeviceCode)
	endpoint, err := fixture.store.GetByID(context.Background(), answer.EndpointID)
	if err != nil {
		t.Fatalf("stored endpoint: %v", err)
	}
	account := endpoint.Account()
	if account.Email().String() != "qoder-user-user-9" {
		t.Fatalf("email = %q, want the synthetic fallback", account.Email().String())
	}
	if account.WorkspaceID() != "user-9" {
		t.Fatalf("workspace = %q, want the poll's user id", account.WorkspaceID())
	}
}

// TestDevicePoll_RefusesTheWrongCaller pins the binding checks before any
// upstream call: a blank or unknown device code, and a code polled against
// another provider's path.
func TestDevicePoll_RefusesTheWrongCaller(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"), providerWithQoderDeviceFlow("qoder-cn"))
	start := startDevice(t, fixture, "qoder")

	cases := []struct {
		name string
		in   OAuthDevicePollInput
	}{
		{"a blank device code", OAuthDevicePollInput{ProviderID: "qoder"}},
		{"an unknown device code", OAuthDevicePollInput{ProviderID: "qoder", DeviceCode: "gone"}},
		{"a foreign provider path", OAuthDevicePollInput{ProviderID: "qoder-cn", DeviceCode: start.DeviceCode}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := fixture.service.DevicePoll(context.Background(), testCase.in); !isValidation(err) {
				t.Fatalf("error = %v, want a validation refusal", err)
			}
		})
	}
	if len(fixture.tokens.devicePollCall) != 0 {
		t.Fatalf("a refused poll still reached the upstream")
	}
}

// TestDevicePoll_UpstreamFailureKeepsTheFlowRetryable pins the failure round: a
// refused poll is an upstream error and the staged state survives, so the modal's
// next attempt can still succeed.
func TestDevicePoll_UpstreamFailureKeepsTheFlowRetryable(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"))
	start := startDevice(t, fixture, "qoder")
	fixture.tokens.devicePollFn = func(string, string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{}, false, errors.New("boom")
	}

	if _, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{
		ProviderID: "qoder", DeviceCode: start.DeviceCode,
	}); err == nil || isValidation(err) {
		t.Fatalf("error = %v, want an upstream failure", err)
	}
	// Reading the state back is the assertion: a failure must leave it staged.
	stagedDeviceState(t, fixture, start.DeviceCode)

	fixture.tokens.devicePollFn = func(string, string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{AccessToken: "dt-late", UserID: "user-1",
			ExpiresAt: testNow.Add(48 * time.Hour)}, false, nil
	}
	if answer := poll(t, fixture, "qoder", start.DeviceCode); answer.Status != "connected" {
		t.Fatalf("Status = %q after a retryable failure, want connected", answer.Status)
	}
}
