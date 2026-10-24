// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_route_cache_test.go
// @for       The short window the route plan reuses one pool read for.
// @uses      context, testing, time, internal/domain.
// @reason    The plan runs once per proxied request, so without a window every request pays a pool read; the test pins both halves, reuse inside the window and a fresh read once it expires.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-24
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestProxyRouteService_Plan_ReusesThePoolReadWithinTheWindow(t *testing.T) {
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	lister := &fakeProxyLister{rows: []domain.Proxy{poolRow(t, "prx_a", time.Hour, true, "ok", "", "")}}
	svc := NewProxyRouteService(ProxyRouteDeps{
		Proxies:  lister,
		Routes:   &fakeRouteStore{parked: map[string]bool{}},
		Settings: &fakePlanSettings{document: planDocument("", "")},
		Opener:   &fakePlanOpener{},
		Clock:    func() time.Time { return now },
	})

	for range 2 {
		if _, err := svc.Plan(context.Background(), "", "api.destination.example"); err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
	}
	if lister.calls != 1 {
		t.Fatalf("List() calls = %d, want one read inside the window", lister.calls)
	}

	now = now.Add(proxyListCacheTTL)
	if _, err := svc.Plan(context.Background(), "", "api.destination.example"); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if lister.calls != 2 {
		t.Fatalf("List() calls = %d, want a fresh read once the window expires", lister.calls)
	}
}
