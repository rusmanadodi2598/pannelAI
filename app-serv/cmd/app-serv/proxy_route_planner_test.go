// Command app-serv wires the process-wide egress policy.
//
// @file      cmd/app-serv/proxy_route_planner_test.go
// @for       The guarded planner adapter: a planned request has its destination
//
//	validated once, an empty plan does not, and a reported failure
//	reaches the store.
//
// @uses      context, net/netip, testing, time, internal/domain,
//
//	internal/netguard, internal/repository, internal/service.
//
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D8: a proxied request never
//
//	dials its destination, so the dialer's guard sees the proxy's address
//	and the destination check has to happen in the planner, the same
//	rule the static route applies in egressProxy, re-pinned here because
//	a wiring change is exactly what could switch the control off. An
//	empty plan is the shared client's request, whose dialer still
//	validates at connect time, so the adapter must not double-refuse it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package main

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// plannerResolver answers the adapter tests' hostname without touching DNS.
type plannerResolver struct {
	answers map[string][]netip.Addr
}

func (r plannerResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	return r.answers[host], nil
}

// plannerProxyLister serves one pool row set.
type plannerProxyLister struct{ rows []domain.Proxy }

func (l plannerProxyLister) List(context.Context) ([]domain.Proxy, error) { return l.rows, nil }

// plannerRouteStore records what the adapter reports.
type plannerRouteStore struct{ parked []string }

func (s *plannerRouteStore) Next(_ context.Context, _ string, ids []string) ([]string, error) {
	return ids, nil
}

func (s *plannerRouteStore) Park(_ context.Context, proxyID string, _ time.Duration) error {
	s.parked = append(s.parked, proxyID)
	return nil
}

func (s *plannerRouteStore) Parked(context.Context, string) (bool, error) { return false, nil }

// plannerSettings answers one stored document.
type plannerSettings struct{ document domain.Settings }

func (s plannerSettings) Settings(context.Context) (domain.Settings, error) {
	return s.document, nil
}

// plannerOpener opens nothing; the adapter test's row carries no password.
type plannerOpener struct{}

func (plannerOpener) Open(string) (string, error) { return "", nil }

// plannerAdapter builds the adapter over a pool row and a document, with the
// guard resolving the destination to the address the case names.
func plannerAdapter(t *testing.T, address string, allowed []string, enabled bool) (guardedProxyPlanner, *plannerRouteStore) {
	t.Helper()
	row, err := domain.NewProxy("prx_a", "pool prx_a", domain.ProxyProtocolHTTP, "prx_a.example.com", 8080, "", "", time.Now())
	if err != nil {
		t.Fatalf("NewProxy() error = %v", err)
	}
	row.SetEnabled(true, time.Now())
	store := &plannerRouteStore{}
	routes := service.NewProxyRouteService(service.ProxyRouteDeps{
		Proxies:  plannerProxyLister{rows: []domain.Proxy{row}},
		Routes:   store,
		Settings: plannerSettings{document: domain.Settings{Network: domain.NetworkSettings{OutboundProxyEnabled: enabled}}},
		Opener:   plannerOpener{},
	})
	guard, err := netguard.NewGuardWithResolver(plannerResolver{answers: map[string][]netip.Addr{
		"destination.example": {netip.MustParseAddr(address)},
	}}, allowed)
	if err != nil {
		t.Fatalf("NewGuardWithResolver() error = %v", err)
	}
	return guardedProxyPlanner{routes: routes, guard: guard}, store
}

// TestGuardedProxyPlanner_ValidatesThePlannedDestination pins the security half
// of D8: a destination the guard refuses stops a plan that would hide it behind
// a proxy, and an allowlisted destination plans normally.
func TestGuardedProxyPlanner_ValidatesThePlannedDestination(t *testing.T) {
	t.Run("a refused destination stops the plan", func(t *testing.T) {
		adapter, _ := plannerAdapter(t, "127.0.0.1", nil, true)
		if _, err := adapter.Plan(context.Background(), "", "destination.example"); err == nil {
			t.Fatal("Plan() = nil error, want the loopback destination refused before any attempt")
		}
	})

	t.Run("an allowlisted destination plans through the pool", func(t *testing.T) {
		adapter, _ := plannerAdapter(t, "127.0.0.1", []string{"127.0.0.0/8"}, true)
		plan, err := adapter.Plan(context.Background(), "", "destination.example")
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if len(plan) == 0 {
			t.Fatal("Plan() = empty, want the pool row planned")
		}
	})
}

// TestGuardedProxyPlanner_EmptyPlanSkipsTheCheck pins the degradation: an empty
// plan is served by the shared client, whose dialer owns the destination check
// at connect time, so the adapter must not refuse it here.
func TestGuardedProxyPlanner_EmptyPlanSkipsTheCheck(t *testing.T) {
	adapter, _ := plannerAdapter(t, "127.0.0.1", nil, false)
	plan, err := adapter.Plan(context.Background(), "", "destination.example")
	if err != nil {
		t.Fatalf("Plan() error = %v, want the disabled proxy to plan nothing without a check", err)
	}
	if len(plan) != 0 {
		t.Fatalf("Plan() = %v, want empty", plan)
	}
}

// TestGuardedProxyPlanner_ReportFailureReachesTheStore pins that the adapter is
// a pass-through for the parking half too, so a dead candidate is remembered.
func TestGuardedProxyPlanner_ReportFailureReachesTheStore(t *testing.T) {
	adapter, store := plannerAdapter(t, "127.0.0.1", []string{"127.0.0.0/8"}, true)
	adapter.ReportFailure(context.Background(), "prx_a")
	if len(store.parked) != 1 || store.parked[0] != "prx_a" {
		t.Fatalf("parked = %v, want [prx_a]", store.parked)
	}
}
