// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_refresh_worker.go
// @for       The background OAuth token refresher: sweep due tokens on a
//
//	tick, retry with exponential backoff and jitter, dead-letter
//	after the stated attempts (SYSTEM_MAP §6, P2).
//
// @uses      context, log/slog, math/rand, strconv, sync, time,
//
//	internal/domain.
//
// @reason    AGENTS.md §1.6 forbids "just retry forever" and "no retry" as
//
//	unstated defaults, so the policy is explicit here: one sweep
//	per tick refreshes every due token of every code-flow
//	provider, a failing endpoint backs off exponentially with
//	jitter, and the fifth consecutive failure marks the endpoint
//	error (the dead letter) instead of spending another grant.
//	The sweep is a method so the policy is testable without a
//	ticker, and Run is only the clock that drives it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"log/slog"
	mathrand "math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// The refresh policy, stated once (AGENTS.md §1.6): base backoff doubling per
// attempt with jitter, capped, and a dead letter after five attempts.
const (
	oauthRefreshMaxAttempts    = 5
	oauthRefreshBackoffBase    = 30 * time.Second
	oauthRefreshBackoffCeiling = 30 * time.Minute
)

// OAuthRefreshWorker refreshes due OAuth tokens in the background.
type OAuthRefreshWorker struct {
	flow  *OAuthFlowService
	index ProviderIndex

	mu       sync.Mutex
	attempts map[string]int
	rng      *mathrand.Rand
	sleep    func(time.Duration)
}

// NewOAuthRefreshWorker builds the worker over a flow service and the provider
// index it sweeps.
func NewOAuthRefreshWorker(flow *OAuthFlowService, index ProviderIndex) *OAuthRefreshWorker {
	return &OAuthRefreshWorker{
		flow: flow, index: index,
		attempts: map[string]int{},
		rng:      mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
		sleep:    time.Sleep,
	}
}

// Run sweeps on the interval until the context is cancelled; that cancellation
// is the worker's explicit termination condition (AGENTS.md §1.6). The panic
// boundary lives in the composition root's runSupervised, which is where every
// worker in this process gets it.
func (w *OAuthRefreshWorker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshed := w.Sweep(ctx)
			if refreshed > 0 {
				slog.Info("oauth refresh worker refreshed tokens", "count", refreshed)
			}
		}
	}
}

// Sweep refreshes every due, active OAuth endpoint of every provider whose
// flow the shared client can serve, and returns how many tokens it refreshed.
// Device and connector flows are skipped: their cadence belongs to their own
// connectors, and a provider with no token endpoint has nothing to refresh.
func (w *OAuthRefreshWorker) Sweep(ctx context.Context) int {
	refreshed := 0
	for _, provider := range w.index.All() {
		oauth := provider.OAuth
		if oauth == nil || oauth.RequiresCustomExchange() || oauth.TokenURL == "" {
			continue
		}
		status, err := w.flow.Status(ctx, provider.ID)
		if err != nil {
			slog.Warn("oauth refresh worker could not read provider status",
				"provider", provider.ID, "error", err)
			continue
		}
		for _, endpoint := range status.Endpoints {
			if endpoint.Status != string(domain.UpstreamEndpointActive) ||
				endpoint.RefreshState != domain.RefreshDue {
				continue
			}
			if w.sweepOne(ctx, provider.ID, endpoint) {
				refreshed++
			}
		}
	}
	return refreshed
}

// sweepOne refreshes one endpoint and applies the retry policy on failure:
// count the attempt, back off below the limit, dead-letter at it. It reports
// whether the token was refreshed.
func (w *OAuthRefreshWorker) sweepOne(ctx context.Context, providerID string, endpoint OAuthEndpointState) bool {
	_, err := w.flow.Refresh(ctx, providerID, endpoint.EndpointID)
	if err == nil {
		w.mu.Lock()
		delete(w.attempts, endpoint.EndpointID)
		w.mu.Unlock()
		return true
	}

	w.mu.Lock()
	attempt := w.attempts[endpoint.EndpointID] + 1
	w.attempts[endpoint.EndpointID] = attempt
	w.mu.Unlock()
	slog.Warn("oauth refresh attempt failed",
		"provider", providerID, "endpoint", endpoint.EndpointID,
		"attempt", attempt, "max_attempts", oauthRefreshMaxAttempts, "error", err)

	if attempt >= oauthRefreshMaxAttempts {
		message := "oauth refresh failed " + strconv.Itoa(attempt) + " times: " + err.Error()
		if deadErr := w.flow.MarkRefreshDeadLetter(ctx, endpoint.EndpointID, message); deadErr != nil {
			slog.Error("oauth refresh worker could not dead-letter the endpoint",
				"endpoint", endpoint.EndpointID, "error", deadErr)
		}
		w.mu.Lock()
		delete(w.attempts, endpoint.EndpointID)
		w.mu.Unlock()
		return false
	}
	w.sleep(w.backoffDelay(attempt))
	return false
}

// backoffDelay returns the sleep before the next attempt: the base doubled once
// per attempt already spent, capped at the ceiling, with up to a quarter of the
// delay added as jitter while the delay is still below the cap.
//
// Jitter is skipped once the cap is reached, so the ceiling is a real ceiling
// rather than a value the random term can cross, and the jitter window is
// narrower than one doubling step, so the delay never shrinks from one attempt
// to the next however many attempts a failing endpoint accumulates.
func (w *OAuthRefreshWorker) backoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := oauthRefreshBackoffBase
	for step := 1; step < attempt; step++ {
		if delay >= oauthRefreshBackoffCeiling/2 {
			return oauthRefreshBackoffCeiling
		}
		delay *= 2
	}
	if delay >= oauthRefreshBackoffCeiling {
		return oauthRefreshBackoffCeiling
	}
	span := delay / 4
	if span <= 0 {
		return delay
	}
	return delay + time.Duration(w.rng.Int63n(int64(span)+1))
}
