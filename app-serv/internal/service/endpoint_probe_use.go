// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_probe_use.go
// @for       The endpoint connectivity test: choosing the key to probe, opening
//
//	its credential, and recording the outcome on the aggregate.
//
// @uses      internal/domain, context, strings, time.
// @reason    SPEC-API-001 §7.5 tests "does this credential work right now", which
//
//	is a different question from "which key would routing spend": a key
//	parked in its circuit-breaker backoff is skipped by selection and is
//	exactly what an operator re-tests after fixing it. Keeping that
//	distinction, the credential opening, and the outcome recording in
//	one file is what stops the panel and the router from disagreeing; it
//	is separate from endpoint.go because AGENTS.md §1.1 caps a file at
//	250 lines and the CRUD surface already fills one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Test probes one endpoint's credential and records the outcome on the aggregate.
//
// An explicit keyID targets that key; otherwise the first active key is used, so
// the button tests what routing would actually spend (§7.5).
//
// A probe that reaches the upstream and is refused is recorded and returned as a
// normal result, not as an error: "the upstream rejected this credential" is the
// answer the operator asked for, and rendering it as a 500 would hide it.
func (s *EndpointService) Test(ctx context.Context, id, keyID string) (domain.UpstreamEndpoint, ProbeOutcome, error) {
	endpoint, err := s.store.GetByID(ctx, id)
	if err != nil {
		return domain.UpstreamEndpoint{}, ProbeOutcome{}, err
	}
	if s.prober == nil {
		return domain.UpstreamEndpoint{}, ProbeOutcome{}, domain.NewInternalError("connectivity testing is unavailable")
	}

	now := s.clock()
	key, err := selectProbeKey(endpoint, keyID)
	if err != nil {
		return domain.UpstreamEndpoint{}, ProbeOutcome{}, err
	}
	credential, err := s.credentialFor(endpoint, key)
	if err != nil {
		return domain.UpstreamEndpoint{}, ProbeOutcome{}, err
	}

	probeCtx, cancel := context.WithTimeout(ctx, connectivityProbeTimeout)
	defer cancel()
	outcome, err := s.prober.ProbeEndpoint(probeCtx, endpoint, key, credential)
	if err != nil {
		// The probe could not run at all — an unknown provider, an unbuildable
		// URL. That is a failure of the test, recorded as one so the panel shows
		// it rather than only the log.
		outcome = ProbeOutcome{State: domain.EndpointTestFail, Message: "the connectivity test could not run"}
	}
	outcome.State = normalizedTestState(outcome.State)
	return s.recordProbe(ctx, endpoint, key, outcome, now)
}

// recordProbe writes a probe's outcome through the aggregate's own methods and
// persists it. Health goes through RecordKeySuccess or RecordKeyFailure so the
// circuit breaker owns that state: a second health field would let the panel and
// the router disagree about whether a key may be spent (§7.5).
func (s *EndpointService) recordProbe(ctx context.Context, endpoint domain.UpstreamEndpoint, key domain.UpstreamKey, outcome ProbeOutcome, now time.Time) (domain.UpstreamEndpoint, ProbeOutcome, error) {
	endpoint.RecordTest(outcome.State, outcome.LatencyMS, outcome.Message, now)
	if key.ID() != "" {
		if outcome.State == domain.EndpointTestOK {
			if _, err := endpoint.RecordKeySuccess(key.ID(), now); err != nil {
				return domain.UpstreamEndpoint{}, ProbeOutcome{}, err
			}
		} else {
			reason := outcome.Message
			if reason == "" {
				reason = "connectivity test failed"
			}
			if _, err := endpoint.RecordKeyFailure(key.ID(), reason, now); err != nil {
				return domain.UpstreamEndpoint{}, ProbeOutcome{}, err
			}
		}
	}
	if err := s.store.Update(ctx, endpoint); err != nil {
		return domain.UpstreamEndpoint{}, ProbeOutcome{}, err
	}
	if key.ID() == "" {
		return endpoint, outcome, nil
	}
	if err := s.store.RecordKeyHealth(ctx, endpoint.Key(key.ID())); err != nil {
		return domain.UpstreamEndpoint{}, outcome, err
	}
	return endpoint, outcome, nil
}

// selectProbeKey resolves which key a probe targets: the named one, or the first
// key in the active status ordered by priority.
//
// It does not use NextKey, because that answers a different question ("which key
// would routing spend now"): a key parked inside its circuit-breaker backoff is
// deliberately skipped there, and testing such a key is exactly what an operator
// does after fixing a credential.
func selectProbeKey(endpoint domain.UpstreamEndpoint, keyID string) (domain.UpstreamKey, error) {
	if strings.TrimSpace(keyID) != "" {
		key := endpoint.Key(keyID)
		if key.ID() == "" {
			return domain.UpstreamKey{}, domain.NewNotFoundError("upstream key not found")
		}
		return key, nil
	}
	keys := endpoint.Keys()
	best := domain.UpstreamKey{}
	for _, key := range keys {
		if key.Status() != domain.UpstreamKeyActive {
			continue
		}
		if best.ID() == "" || key.Priority() < best.Priority() {
			best = key
		}
	}
	if best.ID() == "" {
		return domain.UpstreamKey{}, domain.NewValidationError("this endpoint has no active key to test")
	}
	return best, nil
}

// credentialFor unseals the credential a probe needs.
//
// A no_auth endpoint legitimately has none, so an empty credential is returned
// rather than an error. Any other auth type without a usable credential is a
// validation failure: a test that sent none would report a rejection the operator
// would misread as "my credential is wrong".
func (s *EndpointService) credentialFor(endpoint domain.UpstreamEndpoint, key domain.UpstreamKey) (string, error) {
	switch endpoint.AuthType() {
	case domain.UpstreamAuthNone:
		return "", nil
	case domain.UpstreamAuthOAuth:
		if endpoint.OAuth() == nil || endpoint.OAuth().AccessTokenEncrypted == "" {
			return "", domain.NewValidationError("this endpoint has no credential to test")
		}
		return s.openCredential(endpoint.OAuth().AccessTokenEncrypted)
	default:
		if key.EncryptedValue() == "" {
			return "", domain.NewValidationError("this endpoint has no credential to test")
		}
		return s.openCredential(key.EncryptedValue())
	}
}

// openCredential opens a sealed value, reporting a validation failure rather than
// an internal one: an unreadable stored credential is a fact about the row the
// operator is looking at, and the message must not carry the ciphertext.
func (s *EndpointService) openCredential(sealed string) (string, error) {
	plaintext, err := s.sealer.Open(sealed)
	if err != nil {
		return "", domain.NewValidationError("the stored credential could not be read")
	}
	return plaintext, nil
}

// normalizedTestState forces a probe's state into the two values the aggregate
// stores, so a reporter that said anything else cannot write an unknown state the
// panel would render blank.
func normalizedTestState(state string) string {
	if state == domain.EndpointTestOK {
		return domain.EndpointTestOK
	}
	return domain.EndpointTestFail
}
