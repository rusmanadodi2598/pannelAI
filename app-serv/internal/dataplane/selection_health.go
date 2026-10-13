// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_health.go
// @for       The health accounting a selection produces: the key circuit a served or failed call updates, the parking budget it applies, and its persistence.
// @uses      internal/domain, context.
// @reason    Selection and accounting are two jobs: Select decides who serves the next request from the endpoint's current state, while these writes change that state afterwards. Keeping them apart also keeps the keyless rule readable, because a no_auth selection has no key circuit to write and must not fail the request on a bookkeeping error (G15 in the P2 register).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
)

// RecordSuccess applies a served request to the endpoint's parity state and the
// key's circuit, persisting both. A keyless selection (a no_auth endpoint) has
// no key circuit to write, but its upstream was still reached and served, so the
// endpoint's own connection state is recorded for it too.
func (s *Selector) RecordSuccess(ctx context.Context, selection Selection) error {
	now := s.clock()
	endpoint := selection.Endpoint
	endpoint.RecordUpstreamSuccess(now)
	if selection.Key.ID() != "" {
		updated, err := endpoint.RecordKeySuccess(selection.Key.ID(), now)
		if err != nil {
			return err
		}
		if err := s.endpoints.RecordKeyHealth(ctx, updated); err != nil {
			return err
		}
	}
	return s.endpoints.RecordUpstreamOutcome(ctx, endpoint)
}

// RecordFailure applies a failed attempt to the endpoint's parity state and the
// key's circuit. Request writes nothing, because the same body fails on any key
// and no operator needs to see that as a connection state. Target writes the
// connection state and stops there, because the credential was never sent. A
// keyless selection still records the parity error.
func (s *Selector) RecordFailure(ctx context.Context, selection Selection, reason string, class domain.KeyFailureClass) error {
	if class == domain.KeyFailureRequest {
		return nil
	}
	now := s.clock()
	endpoint := selection.Endpoint
	endpoint.RecordUpstreamError(parityCode(class), reason, now)
	if selection.Key.ID() != "" && class.ImplicatesCredential() {
		updated, err := endpoint.RecordKeyFailure(selection.Key.ID(), reason, class, s.healthPolicy(ctx, endpoint.ProviderID()), now)
		if err != nil {
			return err
		}
		if err := s.endpoints.RecordKeyHealth(ctx, updated); err != nil {
			return err
		}
	}
	return s.endpoints.RecordUpstreamOutcome(ctx, endpoint)
}

// healthPolicy resolves the parking budget for one provider. A nil seam and a
// failed read both answer the documented windows: the key is still parked, at the
// length nobody configured, because a settings read must not decide whether a
// failing credential keeps serving.
func (s *Selector) healthPolicy(ctx context.Context, providerID string) domain.KeyHealthPolicy {
	if s.health == nil {
		return domain.DefaultKeyHealthPolicy()
	}
	policy, err := s.health.KeyHealthPolicy(ctx, providerID)
	if err != nil {
		logx.Degraded(s.logger, "key health policy unavailable, parking fell back to the documented windows", err,
			"provider_id", providerID)
		return domain.DefaultKeyHealthPolicy()
	}
	return policy
}

// parityCode names an endpoint-level failure the way the wire's error taxonomy
// does (§8), so the panel's last-error block and the client's error code agree
// about the same upstream event.
func parityCode(class domain.KeyFailureClass) string {
	if class == domain.KeyFailureRateLimit {
		return CodeRateLimited
	}
	return CodeUpstreamError
}

// PersistKey records a key whose health was already updated in memory, so a
// failure discovered after the response body was read needs no second mutation.
func (s *Selector) PersistKey(ctx context.Context, key domain.UpstreamKey) error {
	return s.endpoints.RecordKeyHealth(ctx, key)
}
