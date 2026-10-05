// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_active_combo_test.go
// @for       The combo name on the in-flight marker the relay leg opens: carried
//
//	on every attempted member, and absent on a call that addressed one
//	model.
//
// @uses      context, internal/domain, testing.
// @reason    SPEC-UI-001 §6.5 draws `Client >> Combo >> Gateway >> Upstream`, so
//
//	a request that addressed a combo has to light that combo's node while
//	its members are being called. The relay leg is the only place that
//	knows both names, and the two facts sit next to each other on the
//	marker, so the failure this pins is the quiet one: a marker that
//	named only the member is indistinguishable, on the panel, from a
//	request that addressed that member directly.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package dataplane

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestRelayOnce_MarksEachComboMemberItAttempts pins the failover path: a combo
// that tries a second provider marks that provider too, so the drawing shows the
// provider actually being called rather than the first one it named.
func TestRelayOnce_MarksEachComboMemberItAttempts(t *testing.T) {
	active := &activeRecorder{}
	calls := 0
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newEngineWithTracker(t, server.URL, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/broken", "beta/works"),
	}, active)

	if _, err := engine.Relay(context.Background(), relayRequest("daily"), nil); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	begun, released, stillOpen := active.snapshot()
	if len(begun) != 2 {
		t.Fatalf("markers begun = %v, want one per attempted member", begun)
	}
	if begun[0] != "alpha" || begun[1] != "beta" {
		t.Fatalf("markers begun = %v, want alpha then beta", begun)
	}
	if released != len(begun) {
		t.Fatalf("begun = %d but released = %d: every attempted member must be released", len(begun), released)
	}
	if stillOpen != 0 {
		t.Fatalf("%d markers are still open, want none", stillOpen)
	}
	// Both attempted markers carry the combo the client addressed, not the member
	// each one resolved to: the drawing lights the combo the caller named while it
	// walks the members.
	for _, combo := range active.begunCombos() {
		if combo != "daily" {
			t.Fatalf("marker combo = %q, want daily", combo)
		}
	}
}

// TestRelayOnce_MarksAPlainCallWithNoCombo pins the other half of the same rule:
// a request that addressed one model carries no combo name, so the drawing leaves
// the combo band idle rather than lighting a combo nothing addressed.
func TestRelayOnce_MarksAPlainCallWithNoCombo(t *testing.T) {
	active := &activeRecorder{}
	calls := 0
	server := newRelayUpstream(t, &calls)
	engine := newActiveEngine(t, server.URL, active)

	if _, err := engine.Relay(context.Background(), relayRequest("alpha/works"), nil); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	combos := active.begunCombos()
	if len(combos) != 1 {
		t.Fatalf("markers begun = %d, want one", len(combos))
	}
	if combos[0] != "" {
		t.Fatalf("marker combo = %q, want the empty string", combos[0])
	}
}
