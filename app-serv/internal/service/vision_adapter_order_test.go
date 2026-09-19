// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/vision_adapter_order_test.go
// @for       The vision adapter service's rotation and constructor tests.
// @uses      internal/domain, internal/repository, context, strings, testing,
// @reason    The replacement tests and the rotation/constructor tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the latter moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestVisionAdapterService_Applicable pins the §7.8 decision the data plane
// makes and the rotation state it carries forward.
func TestVisionAdapterService_Applicable(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name        string
		seed        bool
		enabled     bool
		models      []string
		state       domain.RotationState
		wantApplies bool
		wantModels  []string
		wantNext    domain.RotationState
	}{
		{
			name: "disabled", enabled: false, models: nil, wantApplies: false,
		},
		{
			name: "enabled with no models", enabled: true, models: nil, wantApplies: false,
		},
		{
			name: "enabled with one model", enabled: true, models: []string{"openai/gpt-4o"},
			seed: true, wantApplies: true, wantModels: []string{"openai/gpt-4o"},
			wantNext: domain.RotationState{},
		},
		{
			name: "enabled round robin at a stored index", enabled: true, seed: true,
			models:      []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
			state:       domain.RotationState{Index: 1},
			wantApplies: true, wantModels: []string{"openai/gpt-4o-mini", "openai/gpt-4o"},
			wantNext: domain.RotationState{Index: 0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, repo, _ := newAdapterFixture(t, ctx, acceptAll)
			if tc.seed {
				refs := make([]domain.ModelRef, 0, len(tc.models))
				for _, model := range tc.models {
					refs = append(refs, visionRef(t, model))
				}
				// Seeded straight into the fake so the test isolates Applicable
				// from Replace's validation.
				adapter, err := domain.NewVisionAdapter(tc.enabled, true, refs, acceptAll, time.Now().UTC())
				if err != nil {
					t.Fatalf("NewVisionAdapter() error = %v", err)
				}
				repo.adapter = adapter
			}
			order, err := service.Applicable(ctx, tc.state)
			if err != nil {
				t.Fatalf("Applicable() error = %v", err)
			}
			if order.Applies != tc.wantApplies {
				t.Fatalf("Applicable() applies = %t, want %t", order.Applies, tc.wantApplies)
			}
			if !tc.wantApplies {
				return
			}
			if !reflect.DeepEqual(order.Models, tc.wantModels) {
				t.Fatalf("Applicable() = %v, want %v", order.Models, tc.wantModels)
			}
			if order.Next != tc.wantNext {
				t.Fatalf("Applicable() next = %+v, want %+v", order.Next, tc.wantNext)
			}
		})
	}
}

// TestRejectAllVisionCapability pins the placeholder predicate's direction:
// with the capability data still missing from the registry, "we cannot tell"
// must read as "no".
func TestRejectAllVisionCapability(t *testing.T) {
	for _, raw := range []string{"openai/gpt-4o", "anthropic/claude-3", "openai/gpt-4o-mini"} {
		if RejectAllVisionCapability(visionRef(t, raw)) {
			t.Fatalf("RejectAllVisionCapability(%q) = true, want false until the capability table lands", raw)
		}
	}
}

// TestNewVisionAdapterService_RequiresDeps pins the constructor's validation.
func TestNewVisionAdapterService_RequiresDeps(t *testing.T) {
	catalog := newCatalogFixture(t, context.Background())
	repo := newStubAdapterRepo()
	cases := []struct {
		name    string
		deps    VisionAdapterServiceDeps
		wantErr bool
	}{
		{name: "no repository", deps: VisionAdapterServiceDeps{Catalog: catalog.service}, wantErr: true},
		{name: "no catalog service", deps: VisionAdapterServiceDeps{Repo: repo}, wantErr: true},
		{name: "a nil predicate is filled with the safe default", deps: VisionAdapterServiceDeps{Repo: repo, Catalog: catalog.service}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, err := NewVisionAdapterService(tc.deps)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NewVisionAdapterService() accepted %q", tc.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewVisionAdapterService() error = %v", err)
			}
			if service.capable == nil {
				t.Fatal("NewVisionAdapterService() left a nil capability predicate")
			}
		})
	}
}
