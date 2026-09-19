// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_reads_test.go
// @for       The combo service's list/get read-path tests.
// @uses      internal/domain, internal/repository, context, errors, strings,
// @reason    The rotation tests and the read-path tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the latter moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestComboService_Order pins which strategies touch the rotation store and what
// order each returns.
func TestComboService_Order(t *testing.T) {
	ctx := context.Background()
	twoModels := []domain.ComboModel{
		comboRef(t, "openai/gpt-4o", 1),
		comboRef(t, "openai/gpt-4o-mini", 2),
	}
	cases := []struct {
		name      string
		strategy  domain.ComboStrategy
		sticky    int
		judge     string
		modelList []domain.ComboModel
		wantOrder []string
		wantCalls int
	}{
		{
			name: "round_robin rotates", strategy: domain.ComboRoundRobin, sticky: 1, modelList: twoModels,
			wantOrder: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}, wantCalls: 1,
		},
		{
			name: "round_robin with one model does not rotate", strategy: domain.ComboRoundRobin, sticky: 1,
			modelList: twoModels[:1], wantOrder: []string{"openai/gpt-4o"}, wantCalls: 0,
		},
		{
			name: "fallback keeps the stored order", strategy: domain.ComboFallback, modelList: twoModels,
			wantOrder: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}, wantCalls: 0,
		},
		{
			name: "fusion keeps the stored order", strategy: domain.ComboFusion, judge: "openai/gpt-4o",
			modelList: twoModels, wantOrder: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}, wantCalls: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rotation := &stubRotation{}
			service, _ := newComboFixture(t, ctx, rotation)
			combo, err := domain.NewCombo("cmb_test", "ordered", tc.strategy, tc.sticky, tc.judge, tc.modelList, catalogTestNow())
			if err != nil {
				t.Fatalf("NewCombo() error = %v", err)
			}
			order, err := service.Order(ctx, combo)
			if err != nil {
				t.Fatalf("Order() error = %v", err)
			}
			if !equalStrings(order, tc.wantOrder) {
				t.Fatalf("Order() = %v, want %v", order, tc.wantOrder)
			}
			if rotation.calls != tc.wantCalls {
				t.Fatalf("rotation calls = %d, want %d", rotation.calls, tc.wantCalls)
			}
		})
	}
}

// TestComboService_OrderRotatesAcrossRequests proves the wiring advances: three
// calls for a one-sticky round-robin combo must each lead with a different
// model.
func TestComboService_OrderRotatesAcrossRequests(t *testing.T) {
	ctx := context.Background()
	rotation := &stubRotation{}
	service, _ := newComboFixture(t, ctx, rotation)
	combo, err := domain.NewCombo("cmb_test", "rotating", domain.ComboRoundRobin, 1, "", []domain.ComboModel{
		comboRef(t, "a/one", 1), comboRef(t, "b/two", 2), comboRef(t, "c/three", 3),
	}, catalogTestNow())
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	seen := make(map[string]int, 3)
	for range 3 {
		order, err := service.Order(ctx, combo)
		if err != nil {
			t.Fatalf("Order() error = %v", err)
		}
		seen[order[0]]++
	}
	for _, want := range []string{"a/one", "b/two", "c/three"} {
		if seen[want] != 1 {
			t.Fatalf("three rotations led with %v, want each model once", seen)
		}
	}
}

// TestComboService_OrderFallsBackWhenTheStoreFails keeps a request served when
// distribution is unavailable: losing the rotation must not lose the request.
func TestComboService_OrderFallsBackWhenTheStoreFails(t *testing.T) {
	ctx := context.Background()
	rotation := &stubRotation{failure: errors.New("redis is down")}
	service, _ := newComboFixture(t, ctx, rotation)
	combo, err := domain.NewCombo("cmb_test", "rotating", domain.ComboRoundRobin, 1, "", []domain.ComboModel{
		comboRef(t, "a/one", 1), comboRef(t, "b/two", 2),
	}, catalogTestNow())
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	order, err := service.Order(ctx, combo)
	if err != nil {
		t.Fatalf("Order() error = %v, want the stored order when the store fails", err)
	}
	if !equalStrings(order, []string{"a/one", "b/two"}) {
		t.Fatalf("Order() = %v, want the stored order", order)
	}
}

// TestComboService_OrderWithoutAStore documents the nil-store deployment: the
// management plane still serves combo CRUD, and Order serves the stored order.
func TestComboService_OrderWithoutAStore(t *testing.T) {
	ctx := context.Background()
	service, _ := newComboFixture(t, ctx, nil)
	combo, err := domain.NewCombo("cmb_test", "rotating", domain.ComboRoundRobin, 1, "", []domain.ComboModel{
		comboRef(t, "a/one", 1), comboRef(t, "b/two", 2),
	}, catalogTestNow())
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	order, err := service.Order(ctx, combo)
	if err != nil {
		t.Fatalf("Order() error = %v", err)
	}
	if !equalStrings(order, combo.Refs()) {
		t.Fatalf("Order() = %v, want the stored order %v", order, combo.Refs())
	}
}

// TestComboService_ListAndGet cover the read paths a panel table drives.
func TestComboService_ListAndGet(t *testing.T) {
	ctx := context.Background()
	service, _ := newComboFixture(t, ctx, nil)
	seedComboReferences(t, ctx, service)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if _, err := service.Create(ctx, comboDraft(t, name, domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1))); err != nil {
			t.Fatalf("seeding %q: %v", name, err)
		}
	}
	// seedComboReferences creates one combo of its own, so four exist.
	combos, total, err := service.List(ctx, 1, 25)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 4 || len(combos) != 4 {
		t.Fatalf("List() = %d/%d, want 4/4", len(combos), total)
	}
	page, _, err := service.List(ctx, 2, 3)
	if err != nil {
		t.Fatalf("List() page 2 error = %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("List() page 2 = %d combos, want 1", len(page))
	}
	if _, err := service.Get(ctx, "cmb_missing"); !errors.Is(err, domain.ErrComboNotFound) {
		t.Fatalf("Get() = %v, want ErrComboNotFound", err)
	}
	byName, err := service.ByName(ctx, "beta")
	if err != nil {
		t.Fatalf("ByName() error = %v", err)
	}
	if byName.Name() != "beta" {
		t.Fatalf("ByName() = %q, want beta", byName.Name())
	}
}

// TestNewComboService_RequiresDeps pins the constructor's validation and the
// documented optional rotation store.
func TestNewComboService_RequiresDeps(t *testing.T) {
	catalog := newCatalogFixture(t, context.Background())
	cases := []struct {
		name    string
		deps    ComboServiceDeps
		wantErr bool
	}{
		{name: "no repository", deps: ComboServiceDeps{Catalog: catalog.service}, wantErr: true},
		{name: "no catalog service", deps: ComboServiceDeps{Repo: catalog.combos}, wantErr: true},
		{name: "a rotation store is optional", deps: ComboServiceDeps{Repo: catalog.combos, Catalog: catalog.service}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewComboService(tc.deps)
			if tc.wantErr && err == nil {
				t.Fatalf("NewComboService() accepted %q", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("NewComboService() error = %v", err)
			}
		})
	}
}
