// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_device_poll.go
// @for       One poll of a device authorization round, and the connect a
//
//	successful poll lands (draft 036 slice A, SPEC-API-001 §7.4).
//
// @uses      context, encoding/json, strings, internal/domain,
//
//	internal/registry.
//
// @reason    The poll is where a device flow can go wrong quietly: a vendor
//
//	that answers "not yet" with a 404, an identity read that fails after
//	the login already succeeded, a token handed out twice. Each of those
//	lives here, beside the one state rule they all turn on — the staged
//	round is consumed only once the upstream has actually issued.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// OAuthDevicePollInput is one poll attempt: the provider and the device code the
// start returned. The PKCE verifier is not a client secret here — the gateway
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

	token, pending, err := s.tokens.DevicePoll(ctx, oauth.DeviceTokenURL, payload.Nonce, payload.CodeVerifier)
	if err != nil {
		return OAuthDevicePoll{}, err
	}
	if pending {
		return OAuthDevicePoll{Status: deviceFlowPending}, nil
	}

	// Consume before connecting: a token the upstream hands out twice is stored once.
	if _, _, err := s.states.Take(ctx, code); err != nil {
		return OAuthDevicePoll{}, err
	}

	now := s.clock()
	expires := floorDeviceExpiry(token.ExpiresAt, now)
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

// deviceAccount builds the account a device poll connects, fail-open: the
// reference treats its userinfo read as best-effort, so an unreadable identity
// falls back to the synthetic email the dedup match still recognizes instead of
// blocking a login the vendor already granted.
func (s *OAuthFlowService) deviceAccount(ctx context.Context, oauth *registry.OAuth, token DeviceTokenResponse, machineID string) domain.EndpointAccount {
	account := domain.EndpointAccount{
		Name:        token.UserID,
		Email:       deviceUserEmail + token.UserID,
		MachineID:   machineID,
		WorkspaceID: token.UserID,
	}
	if oauth.UserInfoURL == "" || token.AccessToken == "" {
		return account
	}
	identity, err := s.tokens.UserInfo(ctx, oauth.UserInfoURL, token.AccessToken)
	if err != nil {
		return account
	}
	if identity.Name != "" {
		account.Name = identity.Name
	}
	if identity.Email != "" {
		account.Email = identity.Email
	}
	return account
}
