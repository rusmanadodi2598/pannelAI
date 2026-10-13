// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings_health_policy_test.go
// @for       The settings read behind the parking windows: the budget resolved per provider, over a row that predates the keys and over one that configures them.
// @uses      context, testing, time, internal/domain.
// @reason    SPEC-API-001 §7.14 makes the routing group the operator's lever over how long a failing credential is parked, and §7.5 makes both health writers consult this read. Draft 050 T8 is the finding that they had nothing to consult, so the resolution and the defaults a stored row cannot carry are pinned here.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-13
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSettingsService_KeyHealthPolicyResolvesPerProvider is the read the router and
// the Test button share: one stored document, two providers parking differently,
// and a provider with no entry taking the group's windows.
func TestSettingsService_KeyHealthPolicyResolvesPerProvider(t *testing.T) {
	repo := newStubSettingsStore()
	stored := `{"combo_strategy":"fallback","combo_sticky_limit":1,"sticky_limit":3,` +
		`"fallback_strategy":"fill-first","key_park":{"auth_seconds":600,"transient_seconds":60},` +
		`"provider_strategies":{"beta":{"key_park":{"transient_seconds":5}}}}`
	if err := repo.Save(context.Background(), domain.SettingsKeyRouting, stored); err != nil {
		t.Fatalf("seeding the routing row: %v", err)
	}
	svc := newSettingsUpdateFixture(t, repo)

	cases := []struct {
		provider string
		want     domain.KeyHealthPolicy
	}{
		{"alpha", domain.KeyHealthPolicy{AuthWindow: 10 * time.Minute, RateLimitWindow: 2 * time.Second, TransientWindow: time.Minute}},
		{"beta", domain.KeyHealthPolicy{AuthWindow: 10 * time.Minute, RateLimitWindow: 2 * time.Second, TransientWindow: 5 * time.Second}},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			got, err := svc.KeyHealthPolicy(context.Background(), tc.provider)
			if err != nil {
				t.Fatalf("KeyHealthPolicy(%s) error = %v", tc.provider, err)
			}
			if got != tc.want {
				t.Fatalf("KeyHealthPolicy(%s) = %+v, want %+v", tc.provider, got, tc.want)
			}
		})
	}
}

// TestSettingsService_KeyHealthPolicyAnswersDefaultsForAPreChangeRow is what keeps
// an existing deployment whole: a routing row written before the keys existed reads
// back as the documented windows rather than zeros, which would park a failing
// credential for nothing and put it straight back into rotation.
func TestSettingsService_KeyHealthPolicyAnswersDefaultsForAPreChangeRow(t *testing.T) {
	repo := newStubSettingsStore()
	if err := repo.Save(context.Background(), domain.SettingsKeyRouting,
		`{"combo_strategy":"fallback","combo_sticky_limit":1,"sticky_limit":3}`); err != nil {
		t.Fatalf("seeding a pre-change routing row: %v", err)
	}
	svc := newSettingsUpdateFixture(t, repo)

	got, err := svc.KeyHealthPolicy(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("KeyHealthPolicy() error = %v", err)
	}
	if want := domain.DefaultKeyHealthPolicy(); got != want {
		t.Fatalf("KeyHealthPolicy() = %+v, want %+v", got, want)
	}
}

// TestSettingsService_KeyHealthPolicySurfacesAReadFailure keeps the direction the
// rotation read already takes: the service reports the failure, and each caller
// decides its own degradation.
func TestSettingsService_KeyHealthPolicySurfacesAReadFailure(t *testing.T) {
	svc, err := NewSettingsService(SettingsServiceDeps{Repo: failingSettingsStore{}})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}
	if _, err := svc.KeyHealthPolicy(context.Background(), "alpha"); err == nil {
		t.Fatal("KeyHealthPolicy() error = nil, want the read failure")
	}
}
