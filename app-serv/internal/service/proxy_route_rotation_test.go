// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_route_rotation_test.go
// @for       The plan's rotation and degradation halves: round-robin through
//
//	the store, every degrade-instead-of-fail path, the credential
//	composition, and the failure report.
//
// @uses      context, errors, testing, time, internal/domain.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D5/D6: rotation is an
//
//	optimisation and parking is a hint, so each of these paths must
//	answer a working plan rather than an error, the opposite outcome
//	from the refusals the plan table pins, which is why they live in
//	their own file.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestProxyRouteService_PlanRoundRobin(t *testing.T) {
	oldest := poolRow(t, "prx_a", 3*time.Hour, true, "", "", "")
	middle := poolRow(t, "prx_b", 2*time.Hour, true, "", "", "")
	newest := poolRow(t, "prx_c", 1*time.Hour, true, "", "", "")

	t.Run("the store's rotation leads the attempt list", func(t *testing.T) {
		svc := routeFixture([]domain.Proxy{oldest, middle, newest}, planDocument(domain.ProxyStrategyRoundRobin, ""))
		svc.routes.(*fakeRouteStore).nextOrder = []string{"prx_c", "prx_a", "prx_b"}

		plan, err := svc.Plan(context.Background(), "api.destination.example")
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		want := []string{"prx_c", "prx_a", "prx_b"}
		got := attemptIDs(plan)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Plan() ids = %v, want the rotated %v", got, want)
			}
		}
	})

	t.Run("a store failure degrades to the stable order rather than failing the request", func(t *testing.T) {
		svc := routeFixture([]domain.Proxy{oldest, middle, newest}, planDocument(domain.ProxyStrategyRoundRobin, ""))
		svc.routes.(*fakeRouteStore).nextErr = errors.New("redis down")

		plan, err := svc.Plan(context.Background(), "api.destination.example")
		if err != nil {
			t.Fatalf("Plan() error = %v, want a degraded plan", err)
		}
		got := attemptIDs(plan)
		if got[0] != "prx_a" || got[1] != "prx_b" || got[2] != "prx_c" {
			t.Fatalf("Plan() ids = %v, want the stable order", got)
		}
	})
}

func TestProxyRouteService_PlanDegrades(t *testing.T) {
	t.Run("a pool read failure degrades to the shared route", func(t *testing.T) {
		svc := routeFixture(nil, planDocument("", "http://static.example.com:3128"))
		svc.proxies.(*fakeProxyLister).err = errors.New("db down")

		plan, err := svc.Plan(context.Background(), "api.destination.example")
		if err != nil {
			t.Fatalf("Plan() error = %v, want a degraded plan", err)
		}
		if len(plan) != 0 {
			t.Fatalf("Plan() = %v, want empty", plan)
		}
	})

	t.Run("a settings read failure refuses the plan", func(t *testing.T) {
		svc := routeFixture(nil, domain.Settings{})
		svc.settings.(*fakePlanSettings).err = errors.New("db down")

		if _, err := svc.Plan(context.Background(), "api.destination.example"); err == nil {
			t.Fatal("Plan() accepted a request without being able to read the proxy settings")
		}
	})

	t.Run("a strategy outside the closed set refuses the plan", func(t *testing.T) {
		svc := routeFixture([]domain.Proxy{poolRow(t, "prx_a", time.Hour, true, "", "", "")}, planDocument("random", ""))
		if _, err := svc.Plan(context.Background(), "api.destination.example"); err == nil {
			t.Fatal("Plan() accepted an out-of-set strategy")
		}
	})

	t.Run("a candidate whose seal cannot be opened is skipped", func(t *testing.T) {
		rows := []domain.Proxy{
			poolRow(t, "prx_a", 2*time.Hour, true, "", "operator", "sealed-a"),
			poolRow(t, "prx_b", 1*time.Hour, true, "", "operator", "sealed-b"),
		}
		svc := routeFixture(rows, planDocument("", ""))
		svc.opener.(*fakePlanOpener).err = errors.New("seal broken")

		plan, err := svc.Plan(context.Background(), "api.destination.example")
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if got := attemptIDs(plan); len(got) != 0 {
			t.Fatalf("Plan() = %v, want every sealed candidate skipped", got)
		}
	})
}

func TestProxyRouteService_Credentials(t *testing.T) {
	row := poolRow(t, "prx_a", time.Hour, true, "", "operator", "sealed-a")
	svc := routeFixture([]domain.Proxy{row}, planDocument("", ""))

	plan, err := svc.Plan(context.Background(), "api.destination.example")
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("Plan() = %v, want one attempt", plan)
	}
	if got := plan[0].URL.User.String(); got != "operator:opened-sealed-a" {
		t.Fatalf("the dial URL userinfo = %q, want the opened secret", got)
	}
	if plan[0].URL.String() != "http://operator:opened-sealed-a@prx_a.example.com:8080" {
		t.Fatalf("the dial URL = %q", plan[0].URL.String())
	}
}

func TestProxyRouteService_ReportFailure(t *testing.T) {
	svc := routeFixture(nil, planDocument("", ""))
	store := svc.routes.(*fakeRouteStore)

	svc.ReportFailure(context.Background(), "prx_a")
	if len(store.parkedIDs) != 1 || store.parkedIDs[0] != "prx_a" {
		t.Fatalf("ReportFailure parked %v, want [prx_a]", store.parkedIDs)
	}

	// The static member carries no id: parking nothing is the correct report.
	svc.ReportFailure(context.Background(), "")
	if len(store.parkedIDs) != 1 {
		t.Fatalf("ReportFailure parked %v, want no second entry", store.parkedIDs)
	}
}

func TestProxyRouteService_PlanUsesTheDomainExemption(t *testing.T) {
	// The exemption is the destination-based bypass the static route already
	// honours; the pool route must read the same rule from the same place.
	if !domain.ProxyExempt("internal.example.com,*.other.example", "api.internal.example.com") {
		t.Fatal("domain.ProxyExempt lost the NO_PROXY suffix semantics")
	}
	if !domain.ProxyExempt("*", "any.host") {
		t.Fatal("domain.ProxyExempt lost the star exemption")
	}
	if domain.ProxyExempt("internal.example.com", "apiinternal.example.com") {
		t.Fatal("domain.ProxyExempt matched a host whose suffix is not a domain boundary")
	}
}
