// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_route_binding_test.go
// @for       The per-provider binding's table: None, Global, a pinned pool, and
//
//	the strategy override that orders each walk.
//
// @uses      context, testing, time, internal/domain.
// @reason    docs/PORT/009-PORT-PROVIDER-PROXY.md D2-D5/D8: the binding is the
//
//	pass's whole point, so every mode is pinned here: direct on `__none__`,
//	the pin leading the walk, the pin working while the global switch is
//	off, the graceful fallback when the pinned row is unusable or gone,
//	and the per-provider cursor.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// bindingDocument builds a document whose global switch is on, with one
// provider entry.
func bindingDocument(globalStrategy, staticURL, providerID string, entry domain.ProviderProxy) domain.Settings {
	document := planDocument(globalStrategy, staticURL)
	document.Network.ProviderProxies = map[string]domain.ProviderProxy{providerID: entry}
	return document
}

func TestProxyRouteService_Plan_BindingModes(t *testing.T) {
	oldest := poolRow(t, "prx_a", 3*time.Hour, true, "ok", "", "")
	middle := poolRow(t, "prx_b", 2*time.Hour, true, "ok", "", "")
	newest := poolRow(t, "prx_c", 1*time.Hour, true, "ok", "", "")

	cases := []struct {
		name     string
		rows     []domain.Proxy
		document domain.Settings
		provider string
		wantIDs  []string
	}{
		{
			name:     "none dials direct even while the global proxy is on",
			rows:     []domain.Proxy{oldest, middle},
			document: bindingDocument("", "", "openai", domain.ProviderProxy{PoolID: domain.ProxyPoolNone}),
			provider: "openai",
			wantIDs:  []string{},
		},
		{
			name:     "the pin leads the walk and the rest follow in stored order",
			rows:     []domain.Proxy{newest, oldest, middle},
			document: bindingDocument("", "", "openai", domain.ProviderProxy{PoolID: "prx_c"}),
			provider: "openai",
			wantIDs:  []string{"prx_c", "prx_a", "prx_b"},
		},
		{
			name:     "the pin keeps the static url as the last resort while the global switch is on",
			rows:     []domain.Proxy{oldest, middle},
			document: bindingDocument("", "http://static.example.com:3128", "openai", domain.ProviderProxy{PoolID: "prx_b"}),
			provider: "openai",
			wantIDs:  []string{"prx_b", "prx_a", ""},
		},
		{
			name: "a pin works while the global switch is off, because the pin is the instruction",
			rows: []domain.Proxy{oldest, middle, newest},
			document: func() domain.Settings {
				document := bindingDocument("", "http://static.example.com:3128", "openai", domain.ProviderProxy{PoolID: "prx_b"})
				document.Network.OutboundProxyEnabled = false
				return document
			}(),
			provider: "openai",
			wantIDs:  []string{"prx_b", "prx_a", "prx_c"},
		},
		{
			name: "an entry without a pool still obeys the global switch",
			rows: []domain.Proxy{oldest},
			document: func() domain.Settings {
				document := bindingDocument("", "", "openai", domain.ProviderProxy{Strategy: domain.ProxyStrategyRoundRobin})
				document.Network.OutboundProxyEnabled = false
				return document
			}(),
			provider: "openai",
			wantIDs:  []string{},
		},
		{
			name: "an unusable pinned row falls through to the rest",
			rows: []domain.Proxy{
				poolRow(t, "prx_a", 3*time.Hour, false, "ok", "", ""),
				middle,
			},
			document: bindingDocument("", "", "openai", domain.ProviderProxy{PoolID: "prx_a"}),
			provider: "openai",
			wantIDs:  []string{"prx_b"},
		},
		{
			name:     "a pinned id that names no row falls through to the rest",
			rows:     []domain.Proxy{oldest, middle},
			document: bindingDocument("", "", "openai", domain.ProviderProxy{PoolID: "prx_gone"}),
			provider: "openai",
			wantIDs:  []string{"prx_a", "prx_b"},
		},
		{
			name:     "a binding for another provider does not apply",
			rows:     []domain.Proxy{oldest, middle},
			document: bindingDocument("", "", "openai", domain.ProviderProxy{PoolID: domain.ProxyPoolNone}),
			provider: "anthropic",
			wantIDs:  []string{"prx_a", "prx_b"},
		},
		{
			name: "an exempt destination plans nothing even when pinned",
			rows: []domain.Proxy{oldest},
			document: func() domain.Settings {
				document := bindingDocument("", "", "openai", domain.ProviderProxy{PoolID: "prx_a"})
				document.Network.OutboundNoProxy = "destination.example"
				return document
			}(),
			provider: "openai",
			wantIDs:  []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := routeFixture(tc.rows, tc.document)
			plan, err := svc.Plan(context.Background(), tc.provider, "api.destination.example")
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			assertIDs(t, plan, tc.wantIDs)
		})
	}
}

// TestProxyRouteService_Plan_ProviderStrategyOverride pins D3/D8: the entry's
// strategy orders the walk, and the cursor is keyed by the provider so one
// provider's traffic does not advance another's position.
func TestProxyRouteService_Plan_ProviderStrategyOverride(t *testing.T) {
	oldest := poolRow(t, "prx_a", 3*time.Hour, true, "ok", "", "")
	middle := poolRow(t, "prx_b", 2*time.Hour, true, "ok", "", "")
	newest := poolRow(t, "prx_c", 1*time.Hour, true, "ok", "", "")

	document := bindingDocument(domain.ProxyStrategyFallback, "", "openai", domain.ProviderProxy{
		Strategy: domain.ProxyStrategyRoundRobin,
	})
	svc := routeFixture([]domain.Proxy{oldest, middle, newest}, document)
	store := svc.routes.(*fakeRouteStore)
	store.nextOrder = []string{"prx_c", "prx_a", "prx_b"}

	plan, err := svc.Plan(context.Background(), "openai", "api.destination.example")
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	assertIDs(t, plan, []string{"prx_c", "prx_a", "prx_b"})
	if len(store.nextKeys) != 1 || store.nextKeys[0] != "openai" {
		t.Fatalf("cursor keys = %v, want one key named after the provider", store.nextKeys)
	}

	// The global default is fallback, so a provider without an override never
	// touches the cursor: one rotation cannot be shared by accident.
	plain := routeFixture([]domain.Proxy{oldest, middle}, planDocument(domain.ProxyStrategyFallback, ""))
	plainStore := plain.routes.(*fakeRouteStore)
	if _, err := plain.Plan(context.Background(), "anthropic", "api.destination.example"); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(plainStore.nextKeys) != 0 {
		t.Fatalf("cursor keys = %v, want none under fallback", plainStore.nextKeys)
	}

	// An unscoped caller keeps the pass-008 key, so a call without a provider
	// still shares one rotation rather than inventing a key per request.
	unscoped := routeFixture([]domain.Proxy{oldest, middle}, planDocument(domain.ProxyStrategyRoundRobin, ""))
	unscopedStore := unscoped.routes.(*fakeRouteStore)
	if _, err := unscoped.Plan(context.Background(), "", "api.destination.example"); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(unscopedStore.nextKeys) != 1 || unscopedStore.nextKeys[0] != domain.ProxyRoutePoolKey {
		t.Fatalf("cursor keys = %v, want %q", unscopedStore.nextKeys, domain.ProxyRoutePoolKey)
	}
}

// TestProxyRouteService_Plan_PinnedRowStillRotatesTheRest pins D4's round-robin
// half: the pin stays first and the remaining rows follow the cursor's order.
func TestProxyRouteService_Plan_PinnedRowStillRotatesTheRest(t *testing.T) {
	oldest := poolRow(t, "prx_a", 3*time.Hour, true, "ok", "", "")
	middle := poolRow(t, "prx_b", 2*time.Hour, true, "ok", "", "")
	newest := poolRow(t, "prx_c", 1*time.Hour, true, "ok", "", "")

	document := bindingDocument(domain.ProxyStrategyRoundRobin, "", "openai", domain.ProviderProxy{PoolID: "prx_b"})
	svc := routeFixture([]domain.Proxy{oldest, middle, newest}, document)
	store := svc.routes.(*fakeRouteStore)
	store.nextOrder = []string{"prx_c", "prx_b", "prx_a"}

	plan, err := svc.Plan(context.Background(), "openai", "api.destination.example")
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	assertIDs(t, plan, []string{"prx_b", "prx_c", "prx_a"})
}

// TestProxyRouteService_Plan_PinnedRowParkedSkipsToTheRest keeps the pin from
// resurrecting a row the walk itself just parked.
func TestProxyRouteService_Plan_PinnedRowParkedSkipsToTheRest(t *testing.T) {
	oldest := poolRow(t, "prx_a", 3*time.Hour, true, "ok", "", "")
	middle := poolRow(t, "prx_b", 2*time.Hour, true, "ok", "", "")

	document := bindingDocument("", "", "openai", domain.ProviderProxy{PoolID: "prx_a"})
	svc := routeFixture([]domain.Proxy{oldest, middle}, document)
	svc.routes.(*fakeRouteStore).parked = map[string]bool{"prx_a": true}

	plan, err := svc.Plan(context.Background(), "openai", "api.destination.example")
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	assertIDs(t, plan, []string{"prx_b"})
}

func assertIDs(t *testing.T, plan []domain.ProxyRouteAttempt, want []string) {
	t.Helper()
	got := attemptIDs(plan)
	if len(got) != len(want) {
		t.Fatalf("plan ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("plan ids = %v, want %v", got, want)
		}
	}
}
