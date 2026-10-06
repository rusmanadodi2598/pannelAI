// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_device_identity_test.go
// @for       The account identity a device poll connects, at both edges of its fail-open.
// @uses      context, errors, testing, time, internal/service/oauthhttp.
// @reason    The identity is what dedups a reconnect into one endpoint row, so the two edges belong together: a userinfo read that fails must not block a login the vendor already granted, and an address the grammar refuses twice must not be stored as a half account.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-06
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
)

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

// TestDevicePoll_RefusesAnUnusableIdentity pins where the fail-open ends: a vendor
// that names an address the grammar refuses in both places cannot be stored at all,
// and the half account that would land loses the machine and workspace ids the next
// login of the same account is deduped on.
func TestDevicePoll_RefusesAnUnusableIdentity(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"))
	fixture.tokens.devicePollFn = func(string, string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{AccessToken: "dt-x", UserID: "user 9",
			ExpiresAt: testNow.Add(48 * time.Hour)}, false, nil
	}
	fixture.tokens.infoFn = func() (oauthhttp.OAuthIdentity, error) {
		return oauthhttp.OAuthIdentity{Name: "Nine", Email: "nine at example.com"}, nil
	}

	start := startDevice(t, fixture, "qoder")
	if _, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{
		ProviderID: "qoder", DeviceCode: start.DeviceCode,
	}); err == nil {
		t.Fatal("DevicePoll() accepted an account whose identity the grammar refuses twice")
	}
	if len(fixture.store.byID) != 0 {
		t.Fatalf("stored endpoints = %d, want an unusable login stored nowhere", len(fixture.store.byID))
	}
}
