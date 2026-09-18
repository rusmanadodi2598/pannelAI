// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/vision_adapter_test.go
// @for       Table-driven tests for the vision adapter service, including the (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/domain, internal/repository, context, strings, testing,
// @reason    §7.8 accepts a model only when the catalog holds it and the
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubAdapterRepo is an in-memory VisionAdapterRepository. The stored value
// starts as the disabled default, which is what a fresh install serves.
type stubAdapterRepo struct {
	adapter domain.VisionAdapter
	saves   int
}

func newStubAdapterRepo() *stubAdapterRepo {
	return &stubAdapterRepo{adapter: domain.DefaultVisionAdapter()}
}

func (r *stubAdapterRepo) Get(context.Context) (domain.VisionAdapter, error) { return r.adapter, nil }

func (r *stubAdapterRepo) Save(_ context.Context, adapter domain.VisionAdapter) error {
	r.adapter = adapter
	r.saves++
	return nil
}

// newAdapterFixture wires the adapter service over the shared catalog fixture.
func newAdapterFixture(t *testing.T, capable domain.VisionCapabilityCheck) (*VisionAdapterService, *stubAdapterRepo, catalogFixture) {
	t.Helper()
	catalog := newCatalogFixture(t)
	repo := newStubAdapterRepo()
	service, err := NewVisionAdapterService(VisionAdapterServiceDeps{
		Repo: repo, Catalog: catalog.service, Capable: capable,
	})
	if err != nil {
		t.Fatalf("NewVisionAdapterService() error = %v", err)
	}
	return service, repo, catalog
}

// visionRef parses a reference for an adapter request.
func visionRef(t *testing.T, raw string) domain.ModelRef {
	t.Helper()
	ref, err := domain.ParseModelRef(raw)
	if err != nil {
		t.Fatalf("ParseModelRef(%q) error = %v", raw, err)
	}
	return ref
}

// acceptAll is a predicate that accepts every model, so a test can isolate the
// catalog check from the capability check.
func acceptAll(domain.ModelRef) bool { return true }

// TestVisionAdapterService_GetServesTheDefault documents the fresh-install
// shape: a usable configuration, never a 404.
func TestVisionAdapterService_GetServesTheDefault(t *testing.T) {
	service, _, _ := newAdapterFixture(t, acceptAll)
	adapter, err := service.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if adapter.Enabled() || adapter.Active() {
		t.Fatalf("Get() = enabled=%t active=%t, want the disabled default", adapter.Enabled(), adapter.Active())
	}
	if adapter.ModelStrings() == nil || len(adapter.ModelStrings()) != 0 {
		t.Fatalf("Get().ModelStrings() = %v, want an empty non-nil list", adapter.ModelStrings())
	}
}

// TestVisionAdapterService_Replace pins the write path: a catalog model the
// predicate accepts is stored, and every rejection names its cause.
func TestVisionAdapterService_Replace(t *testing.T) {
	openaiOnly := func(ref domain.ModelRef) bool { return ref.ProviderID() == "openai" }
	cases := []struct {
		name       string
		enabled    bool
		roundRobin bool
		models     []string
		capable    domain.VisionCapabilityCheck
		wantErr    string
		wantStored []string
		wantCodes  string
	}{
		{
			name: "an accepted model", enabled: true, roundRobin: true, models: []string{"openai/gpt-4o"},
			capable: openaiOnly, wantStored: []string{"openai/gpt-4o"},
		},
		{
			name: "a custom model the catalog holds", enabled: true, models: []string{"openai/local-embed"},
			capable: acceptAll, wantStored: []string{"openai/local-embed"},
		},
		{
			name: "disabling with an empty list", enabled: false, models: nil,
			capable: acceptAll, wantStored: []string{},
		},
		{
			name: "a model outside the catalog", enabled: true, models: []string{"openai/ghost"},
			capable: acceptAll, wantErr: "unknown model: openai/ghost", wantCodes: "VALIDATION_ERROR",
		},
		{
			name: "a disabled catalog model", enabled: true, models: []string{"anthropic/claude-3"},
			capable: acceptAll, wantErr: "unknown model", wantCodes: "VALIDATION_ERROR",
		},
		{
			name: "a model the predicate rejects", enabled: true, models: []string{"openai/gpt-4o"},
			capable: func(domain.ModelRef) bool { return false },
			wantErr: "not vision-capable: openai/gpt-4o", wantCodes: "VALIDATION_ERROR",
		},
		{
			name: "a nil predicate accepts nothing", enabled: true, models: []string{"openai/gpt-4o"},
			capable: nil, wantErr: "not vision-capable", wantCodes: "VALIDATION_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, repo, _ := newAdapterFixture(t, tc.capable)
			refs := make([]domain.ModelRef, 0, len(tc.models))
			for _, model := range tc.models {
				refs = append(refs, visionRef(t, model))
			}
			adapter, err := service.Replace(context.Background(), tc.enabled, tc.roundRobin, refs)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Replace() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Replace() error = %v, want %q", err, tc.wantErr)
				}
				if code := domain.AsAppError(err).Code; code != tc.wantCodes {
					t.Fatalf("Replace() code = %q, want %q", code, tc.wantCodes)
				}
				if repo.saves != 0 {
					t.Fatalf("Replace() stored a configuration it rejected (%d saves)", repo.saves)
				}
				return
			}
			if err != nil {
				t.Fatalf("Replace() error = %v", err)
			}
			if !reflect.DeepEqual(adapter.ModelStrings(), tc.wantStored) {
				t.Fatalf("Replace() = %v, want %v", adapter.ModelStrings(), tc.wantStored)
			}
			if repo.saves != 1 {
				t.Fatalf("Replace() saves = %d, want 1", repo.saves)
			}
			if !adapter.UpdatedAt().After(time.Time{}) {
				t.Fatalf("Replace() UpdatedAt() = %v, want a set instant", adapter.UpdatedAt())
			}
			stored, err := service.Get(context.Background())
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if stored.Enabled() != tc.enabled || stored.RoundRobin() != tc.roundRobin {
				t.Fatalf("Get() flags = %t/%t, want %t/%t", stored.Enabled(), stored.RoundRobin(), tc.enabled, tc.roundRobin)
			}
		})
	}
}

// TestVisionAdapterService_ReplaceRejectsAZeroRef pins the service's answer for
// a reference the schema layer would never forward. "gpt-4o" is refused at the
// wire boundary by the "provider/model" rule, so the service only ever sees
// parsed references; a zero one cannot be in the catalog and is refused as
// unknown rather than stored.
func TestVisionAdapterService_ReplaceRejectsAZeroRef(t *testing.T) {
	service, repo, _ := newAdapterFixture(t, acceptAll)
	_, err := service.Replace(context.Background(), true, false, []domain.ModelRef{{}})
	if err == nil {
		t.Fatal("Replace() accepted a zero reference")
	}
	if !strings.Contains(err.Error(), "unknown model") {
		t.Fatalf("Replace() error = %v, want an unknown-model refusal", err)
	}
	if code := domain.AsAppError(err).Code; code != "VALIDATION_ERROR" {
		t.Fatalf("Replace() code = %q, want VALIDATION_ERROR", code)
	}
	if repo.saves != 0 {
		t.Fatalf("Replace() stored a configuration it rejected (%d saves)", repo.saves)
	}
}

// TestVisionAdapterService_ReplaceIsIdempotent proves a PUT of the same
// configuration is a no-op in effect, which the panel's save button relies on.
func TestVisionAdapterService_ReplaceIsIdempotent(t *testing.T) {
	service, repo, _ := newAdapterFixture(t, acceptAll)
	ctx := context.Background()
	refs := []domain.ModelRef{visionRef(t, "openai/gpt-4o")}
	first, err := service.Replace(ctx, true, true, refs)
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	second, err := service.Replace(ctx, true, true, refs)
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if !reflect.DeepEqual(first.ModelStrings(), second.ModelStrings()) {
		t.Fatalf("Replace() = %v then %v, want the same list", first.ModelStrings(), second.ModelStrings())
	}
	if repo.saves != 2 {
		t.Fatalf("Replace() saves = %d, want one write per PUT", repo.saves)
	}
}

// TestVisionAdapterService_ReplaceRemovesDroppedModels pins the whole-set
// replacement: a model left out of the PUT is gone, not merged.
func TestVisionAdapterService_ReplaceRemovesDroppedModels(t *testing.T) {
	service, _, _ := newAdapterFixture(t, acceptAll)
	ctx := context.Background()
	if _, err := service.Replace(ctx, true, true,
		[]domain.ModelRef{visionRef(t, "openai/gpt-4o"), visionRef(t, "openai/gpt-4o-mini")}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	adapter, err := service.Replace(ctx, true, true, []domain.ModelRef{visionRef(t, "openai/gpt-4o-mini")})
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if !reflect.DeepEqual(adapter.ModelStrings(), []string{"openai/gpt-4o-mini"}) {
		t.Fatalf("Replace() = %v, want only the submitted model", adapter.ModelStrings())
	}
}
