// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_device_test.go
// @for       The §7.4 device routes' HTTP contract: the round start answers and the verdicts one poll reports (draft 036 slice A).
// @uses      net/http, net/http/httptest, strings, testing, time, internal/registry, internal/service.
// @reason    The panel renders exactly what these two routes write, so the field names, the pending-versus-connected body, and which refusal reaches the operator as a 400 are the contract being tested here, not the flow's rules, which the service tests already hold.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-27
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
)

// deviceProvider mirrors the registry shape a device-flow entry has: a poll URL
// and a login URL, and no authorize endpoint for a browser to follow.
func deviceProvider(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{
			DeviceTokenURL: "https://openapi.example.com/api/v1/deviceToken/poll",
			LoginURL:       "https://qoder.example.com/device/selectAccounts",
			UserInfoURL:    "https://openapi.example.com/api/v1/userinfo",
		},
	}
}

func postDeviceStart(t *testing.T, fixture oauthFixture, providerID string) *httptest.ResponseRecorder {
	t.Helper()
	return doOAuth(t, http.MethodPost, "/api/v1/providers/"+providerID+"/oauth/device/start",
		"", providerID, "", fixture.handler.DeviceStart)
}

func postDevicePoll(t *testing.T, fixture oauthFixture, providerID, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doOAuth(t, http.MethodPost, "/api/v1/providers/"+providerID+"/oauth/device/poll",
		body, providerID, "", fixture.handler.DevicePoll)
}

// TestDeviceStartAnswersTheVerificationRound pins the body the modal needs on
// open: the link to show, the code to display, and the cadence to poll on.
func TestDeviceStartAnswersTheVerificationRound(t *testing.T) {
	fixture := newOAuthFixture(t, "", deviceProvider("qoder"))
	rr := postDeviceStart(t, fixture, "qoder")
	if rr.Code != http.StatusOK {
		t.Fatalf("start = %d (body: %s)", rr.Code, rr.Body.String())
	}

	body := decodeBody(t, rr)
	code, _ := body["device_code"].(string)
	if code == "" {
		t.Fatalf("no device_code in %v", body)
	}
	url, _ := body["verification_url"].(string)
	if !strings.HasPrefix(url, "https://qoder.example.com/device/selectAccounts?") || !strings.Contains(url, "challenge_method=S256") {
		t.Fatalf("verification_url = %q", url)
	}
	if userCode, _ := body["user_code"].(string); userCode != strings.ToUpper(code[:8]) {
		t.Fatalf("user_code = %q, want the device code's first eight characters uppercased", userCode)
	}
	if interval, _ := body["interval_seconds"].(float64); interval != 2 {
		t.Fatalf("interval_seconds = %v, want 2", body["interval_seconds"])
	}
	if expiresIn, _ := body["expires_in"].(float64); expiresIn != 300 {
		t.Fatalf("expires_in = %v, want 300", body["expires_in"])
	}
}

// TestDeviceStartRefusesAProviderThatHasNoDeviceFlow pins the operator-facing
// refusal: a code-flow provider cannot be started on these routes.
func TestDeviceStartRefusesAProviderThatHasNoDeviceFlow(t *testing.T) {
	fixture := newOAuthFixture(t, "", oauthProvider("linear"))
	rr := postDeviceStart(t, fixture, "linear")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("start = %d (body: %s), want 400", rr.Code, rr.Body.String())
	}
	mustErrorCode(t, rr, "VALIDATION_ERROR")
}

// TestDevicePollReportsPendingWithoutAnEndpoint pins the waiting answer: 200,
// pending, and no endpoint for the panel to navigate to.
func TestDevicePollReportsPendingWithoutAnEndpoint(t *testing.T) {
	fixture := newOAuthFixture(t, "", deviceProvider("qoder"))
	code := startDeviceFlow(t, fixture, "qoder")

	rr := postDevicePoll(t, fixture, "qoder", `{"device_code":"`+code+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("poll = %d (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "pending" {
		t.Fatalf("status = %v, want pending", body["status"])
	}
	if _, ok := body["endpoint_id"]; ok {
		t.Fatalf("a pending poll named an endpoint: %v", body)
	}
}

// TestDevicePollConnectedNamesTheLandedEndpoint pins the success answer, and
// that the verifier the upstream was asked with came from the staged round
// rather than from the request body.
func TestDevicePollConnectedNamesTheLandedEndpoint(t *testing.T) {
	fixture := newOAuthFixture(t, "", deviceProvider("qoder"))
	code := startDeviceFlow(t, fixture, "qoder")
	fixture.tokens.devicePollFn = func(_, verifier string) (oauthhttp.DeviceTokenResponse, bool, error) {
		if verifier == "" {
			t.Error("the poll reached the upstream without the staged verifier")
		}
		return oauthhttp.DeviceTokenResponse{
			AccessToken: "dt-issued", UserID: "user-7",
			ExpiresAt: time.Now().Add(48 * time.Hour),
		}, false, nil
	}

	rr := postDevicePoll(t, fixture, "qoder", `{"device_code":"`+code+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("poll = %d (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "connected" {
		t.Fatalf("status = %v, want connected", body["status"])
	}
	if endpointID, _ := body["endpoint_id"].(string); endpointID == "" {
		t.Fatalf("a connected poll named no endpoint: %v", body)
	}
	if hint, _ := body["token_hint"].(string); hint == "" || strings.Contains(hint, "dt-issued") {
		t.Fatalf("token_hint = %q, want a masked hint", hint)
	}
	if created, _ := body["created"].(bool); !created {
		t.Fatalf("created = %v, want a first login to report a new endpoint", body["created"])
	}
	if len(fixture.tokens.devicePollArg) != 2 || fixture.tokens.devicePollArg[0] != code {
		t.Fatalf("the poll spent %v, want the staged device code", fixture.tokens.devicePollArg)
	}
}

// TestDevicePollRefusesABodyWithoutADeviceCode pins the schema gate: the body
// must name the round, and neither an empty object nor an empty body passes.
func TestDevicePollRefusesABodyWithoutADeviceCode(t *testing.T) {
	fixture := newOAuthFixture(t, "", deviceProvider("qoder"))

	for _, body := range []string{"", "{}", `{"device_code":""}`} {
		rr := postDevicePoll(t, fixture, "qoder", body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("poll(%q) = %d (body: %s), want 400", body, rr.Code, rr.Body.String())
		}
		mustErrorCode(t, rr, "VALIDATION_ERROR")
	}
	if len(fixture.tokens.devicePollArg) != 0 {
		t.Fatal("a refused poll still reached the upstream")
	}
}

// TestDevicePollRefusesAnUnknownDeviceCode pins the replay answer: a code the
// gateway never staged, or already spent, is a 400 rather than a retry.
func TestDevicePollRefusesAnUnknownDeviceCode(t *testing.T) {
	fixture := newOAuthFixture(t, "", deviceProvider("qoder"))

	rr := postDevicePoll(t, fixture, "qoder", `{"device_code":"never-minted"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("poll = %d (body: %s), want 400", rr.Code, rr.Body.String())
	}
	mustErrorCode(t, rr, "VALIDATION_ERROR")
}

// startDeviceFlow drives a real start and returns the device code the poll
// comes back with.
func startDeviceFlow(t *testing.T, fixture oauthFixture, providerID string) string {
	t.Helper()
	rr := postDeviceStart(t, fixture, providerID)
	if rr.Code != http.StatusOK {
		t.Fatalf("start = %d (body: %s)", rr.Code, rr.Body.String())
	}
	code, _ := decodeBody(t, rr)["device_code"].(string)
	if code == "" {
		t.Fatal("start returned no device code")
	}
	return code
}
