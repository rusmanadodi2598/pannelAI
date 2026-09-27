// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_device.go
// @for       The device authorization flow: minting the verification round
//
//	and polling it to a connect (draft 036 slice A, SPEC-API-001 §7.4).
//
// @uses      context, crypto/rand, encoding/json, errors, fmt, net/url,
//
//	strings, time, internal/domain, internal/registry, internal/repository.
//
// @reason    A device-flow provider (Qoder) declares no authorize URL, so the
//
//	code flow cannot serve it. The gateway mints the PKCE pair, nonce
//	and machine id itself, hands the panel a device code, and answers
//	one upstream poll per request until the vendor returns a token.
//	The verifier and machine id never leave the gateway: the panel's
//	only credential is the unguessable single-use device code, which
//	is also what binds a poll to the flow that started it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// The device round the panel's modal expects: the operator has five minutes to
// authorize and the modal asks again every two seconds. The staged state outlives
// that window by a minute so a poll landing at the deadline still finds its context.
const (
	deviceFlowTTL     = 6 * time.Minute
	deviceInterval    = 2
	deviceFlowWindow  = 300
	deviceCodeLen     = 8
	deviceUserEmail   = "qoder-user-"
	deviceFlowPending = "pending"
)

// oauthDeviceStatePayload is the private context staged under a device code: the
// verifier only the gateway holds and the machine id every future signed request
// from this account reuses.
type oauthDeviceStatePayload struct {
	ProviderID   string `json:"provider_id"`
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"code_verifier"`
	MachineID    string `json:"machine_id"`
}

// OAuthDeviceStartInput is one start request.
type OAuthDeviceStartInput struct {
	ProviderID string
}

// OAuthDeviceStart is the round the panel renders: the URL it shows as a link
// (never follows), the short code the operator recognizes, the device code every
// poll carries back, and the cadence.
type OAuthDeviceStart struct {
	DeviceCode      string
	VerificationURL string
	UserCode        string
	IntervalSeconds int
	ExpiresIn       int
}

// requireDeviceFlow resolves a provider and refuses every entry the device routes
// cannot serve: no oauth block, a connector-required exchange, a browser authorize
// endpoint (that is the code flow's), or a device block missing its poll or login
// URL. Both routes gate through it so start and poll cannot disagree.
func (s *OAuthFlowService) requireDeviceFlow(providerID string) (registry.Provider, *registry.OAuth, error) {
	name := strings.TrimSpace(providerID)
	entry, ok := s.index.Provider(name)
	if !ok {
		return registry.Provider{}, nil, domain.NewValidationError("unknown provider_id: " + name)
	}
	if entry.OAuth == nil {
		return registry.Provider{}, nil, domain.NewValidationError("provider " + name + " does not declare an oauth flow")
	}
	if entry.OAuth.RequiresCustomExchange() {
		return registry.Provider{}, nil, domain.NewValidationError("provider " + name + " needs a connector for its token exchange")
	}
	if entry.OAuth.AuthorizeURL != "" {
		return registry.Provider{}, nil, domain.NewValidationError("provider " + name + " uses an authorize URL; start it through the code flow")
	}
	if entry.OAuth.DeviceTokenURL == "" || entry.OAuth.LoginURL == "" {
		return registry.Provider{}, nil, domain.NewValidationError("provider " + name + " does not declare a complete device flow")
	}
	return entry, entry.OAuth, nil
}

// DeviceStart mints the verification round: a fresh PKCE pair, nonce and machine
// id; the vendor's device page URL carrying the S256 challenge; and a staged
// state keyed by the nonce, which the panel holds as its device code.
func (s *OAuthFlowService) DeviceStart(ctx context.Context, in OAuthDeviceStartInput) (OAuthDeviceStart, error) {
	provider, oauth, err := s.requireDeviceFlow(in.ProviderID)
	if err != nil {
		return OAuthDeviceStart{}, err
	}

	nonce, err := newUUIDv4()
	if err != nil {
		return OAuthDeviceStart{}, domain.NewInternalError("the device nonce could not be generated")
	}
	machineID, err := newUUIDv4()
	if err != nil {
		return OAuthDeviceStart{}, domain.NewInternalError("the device machine id could not be generated")
	}
	verifier, err := randomToken(32)
	if err != nil {
		return OAuthDeviceStart{}, domain.NewInternalError("the PKCE verifier could not be generated")
	}

	payload, err := json.Marshal(oauthDeviceStatePayload{
		ProviderID: provider.ID, Nonce: nonce, CodeVerifier: verifier, MachineID: machineID,
	})
	if err != nil {
		return OAuthDeviceStart{}, domain.NewInternalError("the device flow state could not be encoded")
	}
	if err := s.states.Stage(ctx, nonce, payload, deviceFlowTTL); err != nil {
		if errors.Is(err, repository.ErrStateAlreadyStaged) {
			return OAuthDeviceStart{}, domain.NewConflictError("a device flow is already in flight")
		}
		return OAuthDeviceStart{}, err
	}

	params := url.Values{
		"challenge":        {pkceChallenge(verifier)},
		"challenge_method": {"S256"},
		"machine_id":       {machineID},
		"nonce":            {nonce},
	}
	return OAuthDeviceStart{
		DeviceCode:      nonce,
		VerificationURL: oauth.LoginURL + "?" + params.Encode(),
		UserCode:        userCodeOf(nonce),
		IntervalSeconds: deviceInterval,
		ExpiresIn:       deviceFlowWindow,
	}, nil
}

// userCodeOf shortens a nonce to the code the panel shows: it is display-only,
// never sent upstream, so an operator can recognize their own round.
func userCodeOf(nonce string) string {
	if len(nonce) < deviceCodeLen {
		return strings.ToUpper(nonce)
	}
	return strings.ToUpper(nonce[:deviceCodeLen])
}

// newUUIDv4 formats sixteen random bytes as the canonical UUID v4 shape the
// reference's nonce and machine ids use. The standard library covers it; a
// dependency would not.
func newUUIDv4() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}
