// The state-round half of the handler fixture: a token client that speaks a
// vendor-minted round.
//
// @file      internal/handler/oauth_state_stub_test.go
// @for       Answers the StateRoundClient seam for handler tests that drive a vendor-minted round.
// @uses      context, internal/registry, internal/service.
// @reason    The seam is optional on purpose, so the shared stub does not carry it; a test that
//
//	starts a state round needs one client that does, and it answers a fixed round because
//	what these tests pin is the wire body the panel receives, not the vendor's cadence.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-29
package handler

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
)

// stateVendorRound is the round this stub mints: a vendor state as the handle, the
// vendor's own authorization page, and no short code anywhere.
var stateVendorRound = oauthhttp.StateRound{
	State:    "vendor-state-7f3a",
	AuthURL:  "https://www.codebuddy.example.com/auth?state=vendor-state-7f3a",
	Interval: 5 * time.Second,
	Expires:  300,
}

func (s *oauthStubTokens) StateRound(context.Context, *registry.OAuth) (oauthhttp.StateRound, error) {
	return stateVendorRound, nil
}

func (s *oauthStubTokens) StatePoll(context.Context, *registry.OAuth, string) (oauthhttp.DeviceTokenResponse, bool, error) {
	return oauthhttp.DeviceTokenResponse{}, true, nil
}

func (s *oauthStubTokens) StateRefresh(context.Context, *registry.OAuth, string) (oauthhttp.TokenResponse, error) {
	return oauthhttp.TokenResponse{AccessToken: "at-issued", RefreshToken: "rt-issued", ExpiresIn: 3600}, nil
}

// stateRoundProvider mirrors the registry shape a vendor-minted round declares: a
// state endpoint and a token endpoint, and no authorize URL for a browser.
func stateRoundProvider(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{
			StateURL: "https://www.codebuddy.example.com/v2/plugin/auth/state",
			TokenURL: "https://www.codebuddy.example.com/v2/plugin/auth/token",
		},
	}
}
