// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_relay_bookkeeping_test.go
// @for       The served-answer rule: a health write that fails after the upstream answered must not replace the answer with an error.
// @uses      context, errors, strings, testing, internal/domain, internal/registry.
// @reason    The failure path already swallows its bookkeeping error (recordFailure); the success path returned it, so a Postgres blip or a key deleted mid-request turned a served 200 into a client error.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package dataplane

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestRelayServesTheAnswerWhenTheHealthWriteFails pins that a bookkeeping
// failure after a successful upstream call does not fail the request: the
// answer the upstream produced is what the client receives.
func TestRelayServesTheAnswerWhenTheHealthWriteFails(t *testing.T) {
	calls := 0
	upstream := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-1", "alpha")}
	repo.healthErr = errors.New("the health write failed")
	engine := newEngineWith(t, []registry.Provider{relayProvider("alpha", upstream.URL)},
		repo, map[string]domain.Combo{"combo": comboRow("combo", "alpha/works")}, nil)

	outcome, err := engine.Relay(context.Background(), relayRequest("combo"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v, want the served answer", err)
	}
	if !strings.Contains(string(outcome.Body), "pong") {
		t.Fatalf("body = %q, want the upstream's answer", outcome.Body)
	}
}
