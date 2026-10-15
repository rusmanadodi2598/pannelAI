// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_health_virtual_test.go
// @for       The health writes a virtual endpoint must skip: no row exists, so the statement would match nothing and the driver would answer NOT_FOUND.
// @uses      context, testing, time, internal/domain.
// @reason    The OpenCode free lane synthesizes its endpoint per request, and every served call on it logged "key health could not be recorded after a served call" because RecordUpstreamOutcome's UPDATE matched zero rows and the driver reported NOT_FOUND. markSelected already guarded the virtual prefix; the two outcome writes must guard it the same way, and the reason has to be pinned so a later edit does not "restore" the write.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// virtualSelection builds the selection the free lane presents: a synthesized
// endpoint with no key, exactly as selection_virtual.go builds it.
func virtualSelection(t *testing.T, providerID string) Selection {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(
		VirtualEndpointIDPrefix+providerID, providerID, VirtualEndpointLabel,
		domain.UpstreamAuthNone, 1, now,
	)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint(%s) error = %v", providerID, err)
	}
	return Selection{Endpoint: endpoint}
}

// TestSelector_VirtualEndpointSkipsTheHealthWrites pins that a served or failed
// call on a synthesized endpoint writes nothing and reports no error, so the
// empty result the database would answer never surfaces as a bookkeeping
// failure beside a call that actually served.
func TestSelector_VirtualEndpointSkipsTheHealthWrites(t *testing.T) {
	repo := newMemEndpointRepo()
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection := virtualSelection(t, "opencode")

	if err := selector.RecordSuccess(context.Background(), selection); err != nil {
		t.Fatalf("RecordSuccess() error = %v, want the virtual endpoint skipped", err)
	}
	if err := selector.RecordFailure(context.Background(), selection, "rate limited",
		domain.KeyFailureRateLimit, 0); err != nil {
		t.Fatalf("RecordFailure() error = %v, want the virtual endpoint skipped", err)
	}
	selector.markSelected(context.Background(), selection)

	if len(repo.outcome) != 0 {
		t.Fatalf("parity writes = %d, want none for a synthesized endpoint", len(repo.outcome))
	}
	if len(repo.health) != 0 {
		t.Fatalf("key-health writes = %d, want none for a synthesized endpoint", len(repo.health))
	}
	if repo.stampsSelected != 0 {
		t.Fatalf("selection stamps = %d, want none for a synthesized endpoint", repo.stampsSelected)
	}
}

// TestSelector_StoredEndpointStillWritesHealth is the control: a stored
// endpoint's outcome is still recorded, so the guard narrows the write to the
// synthesized case rather than dropping the accounting.
func TestSelector_StoredEndpointStillWritesHealth(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
		buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
	}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}
	if err := selector.RecordSuccess(context.Background(), selection); err != nil {
		t.Fatalf("RecordSuccess() error = %v", err)
	}
	if len(repo.outcome) != 1 {
		t.Fatalf("parity writes = %d, want the stored endpoint's outcome recorded", len(repo.outcome))
	}
}
