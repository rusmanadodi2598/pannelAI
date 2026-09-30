// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_device_state_round_test.go
// @for       The wire body a vendor-minted (state) device round answers with, next to the PKCE round's.
// @uses      net/http, strings, testing, internal/handler.
// @reason    A state round has no short code at all, and the panel once refused to render one
//
//	because the route still promised a `user_code`: the field is conditional now, so the
//	absence is the contract being pinned here, alongside the handle and the link the panel
//	does need. The PKCE round's code is pinned by TestDeviceStartAnswersTheVerificationRound.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-29
package handler

import (
	"net/http"
	"strings"
	"testing"
)

func TestDeviceStartAnswersAVendorMintedRoundCarriesNoShortCode(t *testing.T) {
	fixture := newOAuthFixture(t, "", stateRoundProvider("codebuddy-intl"))
	rr := postDeviceStart(t, fixture, "codebuddy-intl")
	if rr.Code != http.StatusOK {
		t.Fatalf("start = %d (body: %s)", rr.Code, rr.Body.String())
	}

	body := decodeBody(t, rr)
	if _, ok := body["user_code"]; ok {
		t.Fatalf("body carries user_code (%v): a state round has no short code, and handing the panel an empty one fails its contract", body)
	}
	if code, _ := body["device_code"].(string); code != stateVendorRound.State {
		t.Fatalf("device_code = %q, want the vendor state %q: it is the handle every later poll carries", code, stateVendorRound.State)
	}
	url, _ := body["verification_url"].(string)
	if !strings.HasPrefix(url, "https://www.codebuddy.example.com/auth?state=") {
		t.Fatalf("verification_url = %q, want the vendor's own authorization page", url)
	}
	if interval, _ := body["interval_seconds"].(float64); interval != 5 {
		t.Fatalf("interval_seconds = %v, want the round's 5 seconds", body["interval_seconds"])
	}
	if expiresIn, _ := body["expires_in"].(float64); expiresIn != 300 {
		t.Fatalf("expires_in = %v, want the round's 300 second window", body["expires_in"])
	}
}
