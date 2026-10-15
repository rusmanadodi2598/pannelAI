// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_health.go
// @for       The health accounting a selection produces: the key circuit a served or failed call updates, the parking budget it applies, and its persistence.
// @uses      internal/domain, internal/logx, context, strings, time.
// @reason    Selection and accounting are two jobs: Select decides who serves the next request from the endpoint's current state, while these writes change that state afterwards. Keeping them apart also keeps the keyless rule readable, because a no_auth selection has no key circuit to write and must not fail the request on a bookkeeping error (G15 in the P2 register).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package dataplane

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
)

// healthWriteTimeout bounds one health write that runs after its call is over.
const healthWriteTimeout = 5 * time.Second

// accountingCtx detaches a health write from the caller's cancellation: the write
// describes a call that already happened, so a client hanging up as the answer
// lands must not erase it (draft 021 F6's rule for the usage rows, applied here).
func accountingCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), healthWriteTimeout)
}

// markSelected records that rotation chose this credential for one call. It claims
// no health outcome and parks nothing: the call it announces has not happened.
//
// It runs detached like the outcome writes (draft 050 L1), so a client that hangs up
// cannot erase the step rotation took. A lost stamp costs the next selector one
// ordering guess, not the caller's request, so a failure is logged and dropped.
//
// A virtual endpoint is skipped: the lane synthesizes its account, so no row exists
// to stamp and every request would pay a primary-key update that changes nothing.
func (s *Selector) markSelected(ctx context.Context, selection Selection) {
	if isVirtualEndpoint(selection.Endpoint.ID()) {
		return
	}
	writeCtx, cancel := accountingCtx(ctx)
	defer cancel()

	if err := s.endpoints.MarkSelected(writeCtx, selection.Endpoint.ID(), selection.Key.ID(), s.clock()); err != nil {
		logx.Degraded(s.logger, "rotation could not stamp the chosen credential", err,
			"endpoint_id", selection.Endpoint.ID(), "key_id", selection.Key.ID())
	}
}

// RecordSuccess applies a served request to the endpoint's parity state and the
// key's circuit, persisting both. A keyless selection (a no_auth endpoint) has
// no key circuit to write, but its upstream was still reached and served, so the
// endpoint's own connection state is recorded for it too.
//
// A virtual endpoint is skipped, as markSelected skips it: the lane synthesized
// its account, so the parity update matches no row and the driver answers
// NOT_FOUND, which the caller would report as a bookkeeping failure for a call
// that actually served. The synthesized account is also rebuilt per request, so
// there is no state a write could carry forward.
func (s *Selector) RecordSuccess(ctx context.Context, selection Selection) error {
	if isVirtualEndpoint(selection.Endpoint.ID()) {
		return nil
	}
	writeCtx, cancel := accountingCtx(ctx)
	defer cancel()

	now := s.clock()
	endpoint := selection.Endpoint
	endpoint.RecordUpstreamSuccess(now)
	if selection.Key.ID() != "" {
		updated, err := endpoint.RecordKeySuccess(selection.Key.ID(), now)
		if err != nil {
			return err
		}
		if err := s.endpoints.RecordKeyHealth(writeCtx, updated); err != nil {
			return err
		}
	}
	return s.endpoints.RecordUpstreamOutcome(writeCtx, endpoint)
}

// RecordFailure applies a failed attempt to the endpoint's parity state and the
// key's circuit. Request writes nothing: the same body fails on any key. Target
// writes the connection state only: the credential was never sent.
//
// The unit parked is the unit spent: a key for a keyed endpoint, the account itself
// for one that owns no keys, which is the breaker an OAuth account never had. A 429
// on one key must still not withhold its healthy siblings (draft 050 T1). retryAfter
// is the wait the vendor named, so a 429 saying "come back in two minutes" is not
// answered with a two-second park.
func (s *Selector) RecordFailure(
	ctx context.Context,
	selection Selection,
	reason string,
	class domain.KeyFailureClass,
	retryAfter time.Duration,
) error {
	if class == domain.KeyFailureRequest {
		return nil
	}
	// A virtual endpoint is skipped for the same reason RecordSuccess skips it:
	// no row exists, so the write would answer NOT_FOUND for a real upstream
	// failure and log a bookkeeping error that hides the cause.
	if isVirtualEndpoint(selection.Endpoint.ID()) {
		return nil
	}
	writeCtx, cancel := accountingCtx(ctx)
	defer cancel()

	now := s.clock()
	endpoint := selection.Endpoint
	endpoint.RecordUpstreamError(parityCode(class), reason, now)
	// A vendor 429 is also evidence for the published-quota poller, which would
	// otherwise spend a provider call on an account the vendor is throttling. The
	// poller withholds only: the account keeps serving on its other keys (§7.12).
	if class == domain.KeyFailureRateLimit {
		endpoint.MarkQuotaThrottled(now)
	}
	switch {
	case selection.Key.ID() != "" && class.ImplicatesCredential():
		// The budget read is detached too: a client that left must not silently
		// trade the operator's configured window for the documented default.
		updated, err := endpoint.RecordKeyFailure(selection.Key.ID(), reason, class,
			s.healthPolicy(writeCtx, endpoint.ProviderID()).WithRetryAfter(retryAfter), now)
		if err != nil {
			return err
		}
		if err := s.endpoints.RecordKeyHealth(writeCtx, updated); err != nil {
			return err
		}
	case endpoint.ServesItsOwnCredential() && class.ImplicatesCredential():
		// An account that owns no credential row parks as one. The class test gates
		// the settings read too: a network failure cannot produce a park anyway.
		endpoint.RecordAccountFailure(class,
			s.healthPolicy(writeCtx, endpoint.ProviderID()).WithRetryAfter(retryAfter), now)
	}
	return s.endpoints.RecordUpstreamOutcome(writeCtx, endpoint)
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
	switch class {
	case domain.KeyFailureRateLimit:
		return CodeRateLimited
	case domain.KeyFailureBilling:
		return CodeBillingExhausted
	default:
		return CodeUpstreamError
	}
}

// PersistKey records a key whose health was already updated in memory, so a
// failure discovered after the response body was read needs no second mutation.
func (s *Selector) PersistKey(ctx context.Context, key domain.UpstreamKey) error {
	return s.endpoints.RecordKeyHealth(ctx, key)
}
