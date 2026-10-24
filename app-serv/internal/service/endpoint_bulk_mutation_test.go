// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_bulk_mutation_test.go
// @for       The bulk actions over accounts: resolving a selector against the store, the counted guards that refuse it, and the park window it writes.
// @uses      context, fmt, strings, testing, time, internal/domain.
// @reason    A bulk route is the one management path where a wrong set deletes or parks rows nobody pointed at, and the guards that prevent it all live in this file's subject: the count the caller confirmed against the count storage gives, the ceiling that refuses a set too wide with the number it found, and the resolution that must reach past the first page. None of that is provable against a store that ignores pagination, so the fixtures here are deliberately wider than one page.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// seedBulkAccount stores one endpoint directly, bypassing the create path: a resolution
// test needs a known number of rows, and going through Create would spend the same limits
// the test is trying to observe.
func seedBulkAccount(t *testing.T, store *memEndpointStore, id, providerID string, tags ...string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, id, domain.UpstreamAuthNone, 1, testNow)
	if err != nil {
		t.Fatalf("building %s: %v", id, err)
	}
	for _, tag := range tags {
		if _, err := endpoint.AddTag(tag, testNow); err != nil {
			t.Fatalf("tagging %s: %v", id, err)
		}
	}
	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("storing %s: %v", id, err)
	}
	return endpoint
}

func mustStored(t *testing.T, store *memEndpointStore, id string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := store.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("reloading %s: %v", id, err)
	}
	return endpoint
}

func TestBulkStatusDisablesTheResolvedSetAndParksForThePolicyWindow(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	seedBulkAccount(t, store, "ep_a", "deepseek")
	seedBulkAccount(t, store, "ep_b", "deepseek")

	outcome, err := svc.BulkStatus(context.Background(),
		BulkSelector{IDs: []string{"ep_a", "ep_b"}}, "disabled", 0, 2)
	if err != nil {
		t.Fatalf("BulkStatus() error = %v", err)
	}
	if outcome.Matched != 2 || outcome.Changed != 2 {
		t.Fatalf("outcome = %+v, want both rows matched and changed", outcome)
	}
	for _, id := range []string{"ep_a", "ep_b"} {
		stored := mustStored(t, store, id)
		if stored.Status() != domain.UpstreamEndpointDisabled {
			t.Fatalf("%s status = %q, want disabled", id, stored.Status())
		}
		until := stored.DisabledUntil()
		if until == nil || !until.Equal(testNow.Add(24*time.Hour)) {
			t.Fatalf("%s window = %v, want the day an unconfigured provider parks for", id, until)
		}
	}
}

// TestBulkStatusWindowFollowsTheProviderPolicy is the adaptive half of the owner's rule:
// a vendor that publishes a five-hour reset releases the account in five hours, and the
// number comes from the policy rather than from a list of names in this file.
func TestBulkStatusWindowFollowsTheProviderPolicy(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("gemini", "deepseek"), newTestSealer(t), nil)
	svc.health = &stubKeyHealthPolicy{policies: map[string]domain.KeyHealthPolicy{
		"gemini":   {QuotaWindow: 5 * time.Hour},
		"deepseek": {QuotaWindow: 24 * time.Hour},
	}}
	seedBulkAccount(t, store, "ep_gem", "gemini")
	seedBulkAccount(t, store, "ep_deep", "deepseek")

	if _, err := svc.BulkStatus(context.Background(), BulkSelector{All: true}, "disabled", 0, 2); err != nil {
		t.Fatalf("BulkStatus() error = %v", err)
	}
	for id, want := range map[string]time.Duration{"ep_gem": 5 * time.Hour, "ep_deep": 24 * time.Hour} {
		until := mustStored(t, store, id).DisabledUntil()
		if until == nil || !until.Equal(testNow.Add(want)) {
			t.Fatalf("%s window = %v, want %v from its own provider", id, until, want)
		}
	}
}

func TestBulkStatusReleasesAnAccountAndClearsWhatParkingLeft(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("deepseek"), newTestSealer(t), nil)
	endpoint := seedBulkAccount(t, store, "ep_a", "deepseek")
	parked := testNow.Add(30 * time.Minute)
	released := domain.RehydrateUpstreamEndpoint(endpoint.ID(), endpoint.ProviderID(), endpoint.Label(),
		endpoint.AuthType(), endpoint.Priority(), domain.UpstreamEndpointDisabled, nil,
		domain.EndpointAccount{}, domain.EndpointTestStatus{}, &parked, nil, testNow, testNow, nil,
		domain.ServiceAccountRef{}, domain.EndpointParity{ConsecutiveErrors: 3},
		domain.OperatorState{DisabledUntil: &parked})
	store.store(released)

	if _, err := svc.BulkStatus(context.Background(), BulkSelector{IDs: []string{"ep_a"}}, "active", 0, 1); err != nil {
		t.Fatalf("BulkStatus() error = %v", err)
	}
	stored := mustStored(t, store, "ep_a")
	if stored.Status() != domain.UpstreamEndpointActive {
		t.Fatalf("status = %q, want active", stored.Status())
	}
	if stored.DisabledUntil() != nil {
		t.Fatalf("window = %v, want the park cleared rather than left to expire", *stored.DisabledUntil())
	}
	if stored.RateLimitedUntil() != nil {
		t.Fatal("re-enabling left the health park standing")
	}
}

func TestBulkGuardsRefuseBeforeAnythingIsWritten(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("deepseek"), newTestSealer(t), nil)
	for i := range 5 {
		seedBulkAccount(t, store, fmt.Sprintf("ep_%d", i), "deepseek")
	}

	cases := []struct {
		name       string
		sel        BulkSelector
		confirm    int
		want       string
		wantPhrase string
	}{
		{name: "a stale confirmation stops the action", sel: BulkSelector{All: true}, confirm: 4, want: "BULK_TARGET_COUNT_MISMATCH", wantPhrase: "resolves to 5 rows, not the 4"},
		{name: "an empty selection is refused by the selector, not here", sel: BulkSelector{}, confirm: 5, want: "BULK_SELECTOR_REQUIRED", wantPhrase: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.BulkStatus(context.Background(), tc.sel, "disabled", 0, tc.confirm)
			if domain.AsAppError(err).Code != tc.want {
				t.Fatalf("code = %q, want %q", domain.AsAppError(err).Code, tc.want)
			}
			if tc.wantPhrase != "" && !strings.Contains(err.Error(), tc.wantPhrase) {
				t.Fatalf("refusal = %v, want it to say %q", err, tc.wantPhrase)
			}
			for i := range 5 {
				if mustStored(t, store, fmt.Sprintf("ep_%d", i)).Status() != domain.UpstreamEndpointActive {
					t.Fatal("a refused action still moved a row")
				}
			}
		})
	}
}

// TestBulkTargetCeilingNamesTheCountItFound is the refusal an operator can act on: not
// "too many" but how many, so they know how far to narrow the selector.
func TestBulkTargetCeilingNamesTheCountItFound(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("deepseek"), newTestSealer(t), nil)
	for i := range MaxBulkTargets + 3 {
		seedBulkAccount(t, store, fmt.Sprintf("ep_%05d", i), "deepseek")
	}
	_, err := svc.BulkStatus(context.Background(), BulkSelector{All: true}, "disabled", 0, MaxBulkTargets+3)
	appErr := domain.AsAppError(err)
	if appErr.Code != "BULK_TARGET_TOO_LARGE" {
		t.Fatalf("code = %q, want BULK_TARGET_TOO_LARGE", appErr.Code)
	}
	if !strings.Contains(appErr.Message, fmt.Sprintf("%d", MaxBulkTargets+3)) {
		t.Fatalf("refusal = %q, want it to name the count the selector resolved to", appErr.Message)
	}
}

// TestBulkStatusResolvesPastTheFirstPage pins the claim Select-All makes: the panel's page
// is not the set. A selector wider than one storage page still reaches every row.
func TestBulkStatusResolvesPastTheFirstPage(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("deepseek"), newTestSealer(t), nil)
	total := bulkResolvePerPage + 37
	for i := range total {
		seedBulkAccount(t, store, fmt.Sprintf("ep_%04d", i), "deepseek", "walk-me")
	}

	outcome, err := svc.BulkStatus(context.Background(), BulkSelector{Tag: "walk-me"}, "disabled", 0, total)
	if err != nil {
		t.Fatalf("BulkStatus() error = %v", err)
	}
	if outcome.Matched != total || outcome.Changed != total {
		t.Fatalf("outcome = %+v, want all %d rows reached", outcome, total)
	}
	if mustStored(t, store, fmt.Sprintf("ep_%04d", total-1)).Status() != domain.UpstreamEndpointDisabled {
		t.Fatal("the last row past the first page was never touched")
	}
}
