// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_state_fixture_test.go
// @for       The test doubles the state round needs: a vendor that mints rounds,
//
//	and the two provider shapes the flow classification is argued on.
//
// @uses      context, time, internal/registry.
// @reason    A state round is a second flow shape in the same seam, so its fake
//
//	answers live beside the ones that drive it rather than inflating the
//	shared fixture past the AGENTS.md §1.1 budget. Both provider builders
//	come along for the same argument in reverse: which shape counts as
//	"the shared client can serve this" is exactly the distinction under
//	test, so the two fixtures a status assertion compares are stated here
//	together.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package service

import (
	"context"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// StateRound mints a vendor-owned round and records that it was asked, so a flow
// test can assert the panel holds the vendor's state rather than a local nonce.
func (f *fakeTokenClient) StateRound(_ context.Context, oauth *registry.OAuth) (oauthhttp.StateRound, error) {
	f.stateRoundCalls++
	f.stateRoundPlatform = oauth.Platform
	if f.stateRoundFn != nil {
		return f.stateRoundFn(oauth)
	}
	return oauthhttp.StateRound{
		State: "state-issued", AuthURL: "https://vendor.example.com/login",
		Interval: 5 * time.Second, Expires: oauthhttp.StateRoundRetryWindow,
	}, nil
}

// StatePoll records the state the flow spent, so a test can prove the round the
// vendor handed out is the one that comes back.
func (f *fakeTokenClient) StatePoll(_ context.Context, _ *registry.OAuth, state string) (oauthhttp.DeviceTokenResponse, bool, error) {
	f.statePollCall = append(f.statePollCall, state)
	if f.statePollFn != nil {
		return f.statePollFn(state)
	}
	return oauthhttp.DeviceTokenResponse{}, true, nil
}

// StateRefresh records the token the worker sent, so a test can prove the renewal
// went through the header endpoint rather than the form grant.
func (f *fakeTokenClient) StateRefresh(_ context.Context, _ *registry.OAuth, refreshToken string) (oauthhttp.TokenResponse, error) {
	f.stateRefreshCall = append(f.stateRefreshCall, refreshToken)
	if f.stateRefreshFn != nil {
		return f.stateRefreshFn(refreshToken)
	}
	return oauthhttp.TokenResponse{AccessToken: "at-rotated", RefreshToken: "rt-rotated", ExpiresIn: 3600}, nil
}

// providerWithStateFlow mirrors a registry entry shaped like the CodeBuddy
// regions: the round is minted by the vendor at its state endpoint, polled by
// state, and refreshed through a header-carrying endpoint of its own.
func providerWithStateFlow(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{
			BaseURL: "https://vendor.example.com", StateURL: "https://vendor.example.com/state",
			TokenURL: "https://vendor.example.com/token", RefreshURL: "https://vendor.example.com/refresh",
			Platform: "ide", UserAgent: "IDE/2.63.2", PollIntervalMS: 5000,
		},
	}
}

// providerNeedingConnector declares the initiate/poll pair: an exchange the
// shared client cannot shape, which is what "needs a connector" has always meant
// and what the registry's kilocode entry actually declares.
//
// It used to be built on a state URL, but a state round is a flow this gateway
// does serve, minted by the vendor, polled by state, so declaring one no longer
// makes a provider connector territory. Keeping the case meaningful means naming
// the shape that still is.
func providerNeedingConnector(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{
			ClientID: "client-" + id, TokenURL: "https://auth.example.com/token",
			InitiateURL: "https://relay.example.com/codes", PollURLBase: "https://relay.example.com/codes",
		},
	}
}
