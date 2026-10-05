// Package service orchestrates the use cases the gateway exposes.
//
// @file      internal/service/oauth_flow_device_race_test.go
// @for       The device poll that loses the state to a faster consumer.
// @uses      context, testing
// @reason    The poll reads the staged state, asks the vendor for a token, then consumes it.
//
//	Two polls can both read it and both get a token, so the consume's boolean is the only
//	thing that stops the loser from connecting the same credential a second time.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package service

import (
	"context"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"testing"
	"time"
)

func TestDevicePoll_ALostConsumeConnectsNothing(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"))
	fixture.tokens.devicePollFn = func(string, string) (oauthhttp.DeviceTokenResponse, bool, error) {
		return oauthhttp.DeviceTokenResponse{AccessToken: "dt-race", UserID: "user-9",
			ExpiresAt: testNow.Add(72 * time.Hour)}, false, nil
	}
	start := startDevice(t, fixture, "qoder")

	// The state was there for the peek and gone for the take: another poll is
	// connecting this account right now.
	fixture.states.takeMiss = true

	answer, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{
		ProviderID: "qoder", DeviceCode: start.DeviceCode,
	})
	if !isValidation(err) {
		t.Fatalf("DevicePoll() error = %v, want the refusal a consumed device code must give", err)
	}
	if answer.EndpointID != "" || answer.Status != "" {
		t.Fatalf("the losing poll reported %+v, want no connect at all", answer)
	}
}
