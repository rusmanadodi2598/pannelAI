// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_device_poll.go
// @for       One poll of a device authorization round, and the connect a successful poll lands (draft 036 slice A, SPEC-API-001 §7.4).
// @uses      context, encoding/json, strings, internal/domain, internal/registry.
// @reason    The poll is where a device flow can go wrong quietly: a vendor that answers "not yet" with a 404, an identity read that fails after the login already succeeded, a token handed out twice. Each of those lives here, beside the one state rule they all turn on, the staged round is consumed only once the upstream has actually issued.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"encoding/json"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// OAuthDevicePollInput is one poll attempt: the provider and the device code the
// start returned. The PKCE verifier is not a client secret here, the gateway
// staged it and spends it on the upstream call itself.
type OAuthDevicePollInput struct {
	ProviderID string
	DeviceCode string
}

// OAuthDevicePoll is one poll's outcome: pending keeps the modal polling;
// connected names the endpoint the flow landed.
type OAuthDevicePoll struct {
	Status     string
	EndpointID string
	TokenHint  string
	Created    bool
}

// DevicePoll performs exactly one upstream poll per call: pending keeps the flow
// alive and staged, success consumes the state and lands the connect, and every
// failure leaves the staged state retryable. The state is consumed only after the
// upstream returned a token, so a success can never write twice.
func (s *OAuthFlowService) DevicePoll(ctx context.Context, in OAuthDevicePollInput) (OAuthDevicePoll, error) {
	provider, oauth, err := s.requireDeviceFlow(in.ProviderID)
	if err != nil {
		return OAuthDevicePoll{}, err
	}
	code := strings.TrimSpace(in.DeviceCode)
	if code == "" {
		return OAuthDevicePoll{}, domain.NewValidationError("device_code is required")
	}

	raw, ok, err := s.states.Peek(ctx, code)
	if err != nil {
		return OAuthDevicePoll{}, err
	}
	if !ok {
		return OAuthDevicePoll{}, domain.NewValidationError("the device code is unknown, expired, or already used")
	}
	var payload oauthDeviceStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return OAuthDevicePoll{}, domain.NewInternalError("the staged device state could not be decoded")
	}
	if payload.ProviderID != provider.ID {
		return OAuthDevicePoll{}, domain.NewValidationError("the device code belongs to another provider")
	}

	token, pending, err := s.pollRound(ctx, oauth, payload)
	if err != nil {
		return OAuthDevicePoll{}, err
	}
	if pending {
		return OAuthDevicePoll{Status: deviceFlowPending}, nil
	}

	// Consume before connecting: a token the upstream hands out twice is stored once.
	// The boolean is the whole guard, so a poll that arrives after another already
	// took the state stops here instead of connecting the same credential a second
	// time, which for a vendor that names no account identity would mean a second
	// endpoint row for one login.
	if _, taken, err := s.states.Take(ctx, code); err != nil {
		return OAuthDevicePoll{}, err
	} else if !taken {
		return OAuthDevicePoll{}, domain.NewValidationError("the device code is unknown, expired, or already used")
	}

	now := s.clock()
	expires := oauthhttp.FloorDeviceExpiry(token.ExpiresAt, now)
	connect, err := s.connectAccount(ctx, provider.ID, connectTokens{
		AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, ExpiresAt: &expires,
	}, s.deviceAccount(ctx, oauth, token, payload.MachineID), "", now)
	if err != nil {
		return OAuthDevicePoll{}, err
	}
	return OAuthDevicePoll{
		Status: "connected", EndpointID: connect.Endpoint.ID(),
		TokenHint: connect.TokenHint, Created: connect.Created,
	}, nil
}

// pollRound spends one round against the shape its provider declared.
//
// The two shapes differ in what the poll sends and in who minted the handle: the
// PKCE round polls with a nonce and the verifier this service generated, the
// state round polls with the value the vendor handed back at start.
func (s *OAuthFlowService) pollRound(ctx context.Context, oauth *registry.OAuth, payload oauthDeviceStatePayload) (oauthhttp.DeviceTokenResponse, bool, error) {
	if !oauth.StateExchangeFlow() {
		return s.tokens.DevicePoll(ctx, oauth.DeviceTokenURL, payload.Nonce, payload.CodeVerifier)
	}
	client, ok := s.tokens.(oauthhttp.StateRoundClient)
	if !ok {
		return oauthhttp.DeviceTokenResponse{}, false, domain.NewInternalError("the state round needs a token client that speaks it")
	}
	return client.StatePoll(ctx, oauth, payload.State)
}

// deviceAccountEmail names the synthetic account a PKCE device login lands on.
//
// A state round answers no identity at all, and it gets no synthetic email either:
// the reference dedups a connection only when the vendor stated an email
// (connectionsRepo.js:133) and otherwise inserts a fresh row per login. A constant
// stand-in here would be a key every account of the region shares, so the second
// login would spend the first one's credential. The PKCE round does read a user id
// from the vendor, so its long-standing prefix stays exactly as it was, stored
// accounts are matched on this string.
func deviceAccountEmail(oauth *registry.OAuth, userID string) string {
	if oauth.StateExchangeFlow() {
		return ""
	}
	return deviceUserEmail + userID
}

// deviceAccount builds the account a device poll connects, fail-open: the reference
// treats its userinfo read as best-effort, so an unreadable identity falls back to
// whatever the token answer itself stated rather than blocking a login the vendor
// already granted. A vendor that states nothing in either place leaves the account
// with no identity, which is the honest answer and the one that keeps the next login
// a separate account.
func (s *OAuthFlowService) deviceAccount(ctx context.Context, oauth *registry.OAuth, token oauthhttp.DeviceTokenResponse, machineID string) domain.EndpointAccount {
	synthetic := deviceAccountEmail(oauth, token.UserID)
	name, email := token.UserID, synthetic
	if oauth.UserInfoURL != "" && token.AccessToken != "" {
		if identity, err := s.tokens.UserInfo(ctx, oauth.UserInfoURL, token.AccessToken); err == nil {
			if identity.Name != "" {
				name = identity.Name
			}
			if identity.Email != "" {
				email = identity.Email
			}
		}
	}
	account, err := domain.NewEndpointAccount(domain.EndpointAccountInput{
		Name: name, Email: email, MachineID: machineID, WorkspaceID: token.UserID,
	})
	if err == nil {
		return account
	}
	// A vendor that answers an unusable address still has a granted login worth
	// keeping, so the identity falls back to the synthetic one this path would
	// have used with no userinfo at all rather than losing the whole account.
	account, _ = domain.NewEndpointAccount(domain.EndpointAccountInput{
		Name: name, Email: synthetic, MachineID: machineID, WorkspaceID: token.UserID,
	})
	return account
}
