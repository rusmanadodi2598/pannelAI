// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings_rotation_test.go
// @for       The one settings read the data plane's selector makes: the
//
//	credential rotation policy resolved per provider, and its behaviour
//	over a row written before the keys existed.
//
// @uses      context, testing, internal/domain.
// @reason    SPEC-API-001 §7.5 makes the selector consult this read on every
//
//	selection. AGENTS.md §2.1 requires it proven beside the service: the
//	per-provider resolution is what makes the panel's switch govern one
//	provider rather than every request, and the read path must answer
//	the documented defaults for a row that predates the keys instead of
//	handing the selector an unset mode.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSettingsService_RotationPolicyResolvesPerProvider pins the override: two
// providers on one stored document walk their own modes, and a provider with no
// entry inherits the global.
func TestSettingsService_RotationPolicyResolvesPerProvider(t *testing.T) {
	repo := newStubSettingsStore()
	stored := `{"combo_strategy":"fallback","combo_sticky_limit":1,"sticky_limit":5,` +
		`"fallback_strategy":"round-robin","provider_strategies":{"beta":{"fallback_strategy":"fill-first","sticky_limit":1}}}`
	if err := repo.Save(context.Background(), domain.SettingsKeyRouting, stored); err != nil {
		t.Fatalf("seeding the routing row: %v", err)
	}
	svc := newSettingsUpdateFixture(t, repo)

	cases := []struct {
		provider string
		want     domain.RotationPolicy
	}{
		{"alpha", domain.RotationPolicy{Strategy: domain.RotationRoundRobin, StickyLimit: 5}},
		{"beta", domain.RotationPolicy{Strategy: domain.RotationFillFirst, StickyLimit: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			got, err := svc.RotationPolicy(context.Background(), tc.provider)
			if err != nil {
				t.Fatalf("RotationPolicy(%s) error = %v", tc.provider, err)
			}
			if got != tc.want {
				t.Fatalf("RotationPolicy(%s) = %+v, want %+v", tc.provider, got, tc.want)
			}
		})
	}
}

// TestSettingsService_RotationPolicyNormalizesAPreChangeRow pins the read path
// for a row written before §7.14 carried these keys: the group decode replaces
// the group wholesale, so the absent keys arrive as zero values and the selector
// must still receive fill-first with the documented sticky limit.
func TestSettingsService_RotationPolicyNormalizesAPreChangeRow(t *testing.T) {
	repo := newStubSettingsStore()
	if err := repo.Save(context.Background(), domain.SettingsKeyRouting,
		`{"combo_strategy":"fallback","combo_sticky_limit":1,"sticky_limit":3}`); err != nil {
		t.Fatalf("seeding a pre-change routing row: %v", err)
	}
	svc := newSettingsUpdateFixture(t, repo)

	got, err := svc.RotationPolicy(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("RotationPolicy() error = %v", err)
	}
	want := domain.RotationPolicy{Strategy: domain.RotationFillFirst, StickyLimit: domain.DefaultStickyLimit}
	if got != want {
		t.Fatalf("RotationPolicy() = %+v, want %+v", got, want)
	}

	read, err := svc.Settings(context.Background())
	if err != nil {
		t.Fatalf("Settings() error = %v", err)
	}
	if read.Routing.ProviderStrategies == nil {
		t.Fatal("provider_strategies = nil, want an empty map so the panel's read renders {}")
	}
}

// TestSettingsService_RotationPolicySurfacesAReadFailure pins the direction the
// service takes: the failure is returned, not hidden. Degrading to fill-first is
// the selector's rule, so a caller that needs the policy can still tell a failed
// read from a stored one.
func TestSettingsService_RotationPolicySurfacesAReadFailure(t *testing.T) {
	svc, err := NewSettingsService(SettingsServiceDeps{Repo: failingSettingsStore{}})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}

	if _, err := svc.RotationPolicy(context.Background(), "alpha"); err == nil {
		t.Fatal("RotationPolicy() error = nil, want the read failure")
	}
}

// failingSettingsStore is a repository whose read always fails, so the service's
// error path is reachable without a database.
type failingSettingsStore struct{}

func (failingSettingsStore) Load(context.Context) (map[domain.SettingsKey]string, error) {
	return nil, errors.New("settings store unavailable")
}

func (failingSettingsStore) Save(context.Context, domain.SettingsKey, string) error {
	return errors.New("settings store unavailable")
}
