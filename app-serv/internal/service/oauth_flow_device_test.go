// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_device_test.go
// @for       The Qoder device authorization flow: start, poll, and the
//
//	connect the poll lands (draft 036 slice A).
//
// @uses      context, encoding/json, errors, net/url, strings, testing,
//
//	internal/domain, internal/registry.
//
// @reason    The device flow is the first start path the gateway itself mints
//
//	a verification URL for, and the first connect whose identity is
//	fail-open. The table asserts the wire shapes the panel renders
//	(verification URL, user code, cadence) and the state lifecycle the
//	replay guard promises: pending polls re-read, a success consumes
//	exactly once, and a failure leaves the flow retryable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// providerWithQoderDeviceFlow mirrors the registry's qoder entry: device token +
// login + userinfo URLs, no authorize URL, no token URL (so refresh stays
// refused, exactly as the reference treats a device token).
func providerWithQoderDeviceFlow(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{
			DeviceTokenURL: "https://openapi.example.com/api/v1/deviceToken/poll",
			LoginURL:       "https://qoder.example.com/device/selectAccounts",
			UserInfoURL:    "https://openapi.example.com/api/v1/userinfo",
		},
	}
}

// startDevice runs DeviceStart and returns the answer.
func startDevice(t *testing.T, fixture oauthFlowFixture, providerID string) OAuthDeviceStart {
	t.Helper()
	start, err := fixture.service.DeviceStart(context.Background(), OAuthDeviceStartInput{ProviderID: providerID})
	if err != nil {
		t.Fatalf("DeviceStart(%q): %v", providerID, err)
	}
	return start
}

// stagedDeviceState reads back what a start staged under its device code.
func stagedDeviceState(t *testing.T, fixture oauthFlowFixture, code string) oauthDeviceStatePayload {
	t.Helper()
	raw, ok := fixture.states.staged[code]
	if !ok {
		t.Fatalf("no state staged under device code %q", code)
	}
	var payload oauthDeviceStatePayload
	if err := json.Unmarshal(raw.payload, &payload); err != nil {
		t.Fatalf("decoding staged device state: %v", err)
	}
	return payload
}

// poll runs one DevicePoll and fails the test on error.
func poll(t *testing.T, fixture oauthFlowFixture, providerID, code string) OAuthDevicePoll {
	t.Helper()
	answer, err := fixture.service.DevicePoll(context.Background(), OAuthDevicePollInput{ProviderID: providerID, DeviceCode: code})
	if err != nil {
		t.Fatalf("DevicePoll: %v", err)
	}
	return answer
}

// TestDeviceStart_BuildsTheVerificationRound pins what the panel renders: the
// staged verifier hashes to the challenge on the verification URL, the device
// code is the nonce the poll comes back with, and the private context never
// leaves the gateway.
func TestDeviceStart_BuildsTheVerificationRound(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithQoderDeviceFlow("qoder"))
	start := startDevice(t, fixture, "qoder")

	if start.IntervalSeconds != 2 {
		t.Fatalf("IntervalSeconds = %d, want the reference's 2", start.IntervalSeconds)
	}
	if start.ExpiresIn != 300 {
		t.Fatalf("ExpiresIn = %d, want the reference's 300", start.ExpiresIn)
	}
	if start.UserCode != strings.ToUpper(start.DeviceCode[:8]) {
		t.Fatalf("UserCode = %q, want the device code's first 8 characters uppercased", start.UserCode)
	}
	if !strings.Contains(start.VerificationURL, start.DeviceCode) {
		t.Fatalf("verification URL does not carry the nonce: %q", start.VerificationURL)
	}

	parsed, err := url.Parse(start.VerificationURL)
	if err != nil {
		t.Fatalf("verification URL: %v", err)
	}
	if got := parsed.Scheme + "://" + parsed.Host + parsed.Path; got != "https://qoder.example.com/device/selectAccounts" {
		t.Fatalf("verification URL base = %q", got)
	}
	query := parsed.Query()
	if query.Get("challenge_method") != "S256" {
		t.Fatalf("challenge_method = %q, want S256", query.Get("challenge_method"))
	}
	if query.Get("machine_id") == "" {
		t.Fatalf("verification URL is missing machine_id: %q", start.VerificationURL)
	}

	payload := stagedDeviceState(t, fixture, start.DeviceCode)
	if payload.ProviderID != "qoder" || payload.Nonce != start.DeviceCode || payload.MachineID == "" {
		t.Fatalf("staged payload incomplete: %+v", payload)
	}
	if payload.CodeVerifier == "" {
		t.Fatalf("staged payload carries no PKCE verifier")
	}
	if pkceChallenge(payload.CodeVerifier) != query.Get("challenge") {
		t.Fatalf("staged verifier does not hash to the URL's challenge")
	}
	if fixture.states.ttl != deviceFlowTTL {
		t.Fatalf("staged TTL = %v, want %v", fixture.states.ttl, deviceFlowTTL)
	}
}

// TestDeviceStart_RefusesTheWrongProviders pins the eligibility gate: the device
// routes serve exactly the entries whose oauth block declares a device token URL
// and a login URL and no browser authorize endpoint.
func TestDeviceStart_RefusesTheWrongProviders(t *testing.T) {
	cases := []struct {
		name     string
		provider registry.Provider
		ask      string
	}{
		{"unknown provider", providerWithQoderDeviceFlow("qoder"), "nosuch"},
		{"a code-flow provider", providerWithCodeFlow("linear"), "linear"},
		{"a connector provider", providerNeedingConnector("antigravity"), "antigravity"},
		{"no oauth block", registry.Provider{ID: "plain"}, "plain"},
		{"no login URL", registry.Provider{ID: "half", OAuth: &registry.OAuth{
			DeviceTokenURL: "https://openapi.example.com/poll"}}, "half"},
		{"no device token URL", registry.Provider{ID: "otherhalf", OAuth: &registry.OAuth{
			LoginURL: "https://qoder.example.com/device"}}, "otherhalf"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newOAuthFlowFixture(t, testCase.provider)
			if _, err := fixture.service.DeviceStart(context.Background(), OAuthDeviceStartInput{ProviderID: testCase.ask}); !isValidation(err) {
				t.Fatalf("DeviceStart accepted %q: error = %v", testCase.name, err)
			}
		})
	}
}

// isValidation reports whether the error carries the validation code the handler
// maps to 400.
func isValidation(err error) bool {
	var coded *domain.AppError
	return errors.As(err, &coded) && coded.Code == "VALIDATION_ERROR"
}
