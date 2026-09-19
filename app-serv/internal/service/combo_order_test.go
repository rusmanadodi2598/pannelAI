// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_order_test.go
// @for       The combo service's rotation wiring and read-path tests. (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/domain, internal/repository, context, errors, strings,
// @reason    The create/update/delete tests and the rotation tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the rotation and read-path cases moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestComboService_Update pins the patch path, including the rename collision
// and that a rejected change leaves the stored aggregate untouched.
func TestComboService_Update(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		update    ComboDraft
		wantErr   string
		wantStrat domain.ComboStrategy
	}{
		{
			name:      "switching to round_robin",
			update:    comboDraft(t, "daily", domain.ComboRoundRobin, 2, "", comboRef(t, "openai/gpt-4o", 1)),
			wantStrat: domain.ComboRoundRobin,
		},
		{
			name:      "keeping the same name",
			update:    comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o-mini", 1)),
			wantStrat: domain.ComboFallback,
		},
		{
			name:      "switching to fusion with a judge",
			update:    comboDraft(t, "daily", domain.ComboFusion, 0, "openai/gpt-4o", comboRef(t, "openai/gpt-4o", 1)),
			wantStrat: domain.ComboFusion,
		},
		{
			name:    "renaming onto the other combo",
			update:  comboDraft(t, "seeded-combo", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1)),
			wantErr: "already exists",
		},
		{
			name:    "an invalid strategy shape",
			update:  comboDraft(t, "daily", domain.ComboFusion, 0, "", comboRef(t, "openai/gpt-4o", 1)),
			wantErr: "requires judge_model",
		},
		{
			name:    "an unresolvable reference",
			update:  comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "openai/ghost", 1)),
			wantErr: "does not resolve",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := newComboFixture(t, ctx, nil)
			seedComboReferences(t, ctx, service)
			created, err := service.Create(ctx, comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1)))
			if err != nil {
				t.Fatalf("seeding a combo: %v", err)
			}
			updated, err := service.Update(ctx, created.ID(), tc.update)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Update() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Update() error = %v, want %q", err, tc.wantErr)
				}
				stored, err := service.Get(ctx, created.ID())
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
				if stored.Strategy() != domain.ComboFallback || stored.Name() != "daily" {
					t.Fatalf("a rejected Update() changed the combo: %q/%q", stored.Name(), stored.Strategy())
				}
				return
			}
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if updated.Strategy() != tc.wantStrat {
				t.Fatalf("Update() strategy = %q, want %q", updated.Strategy(), tc.wantStrat)
			}
		})
	}
}

// TestComboService_UpdateResetsRotationWhenTheListChanges pins the one failure a
// round-robin combo can produce: a sticky index that addressed the old list.
func TestComboService_UpdateResetsRotationWhenTheListChanges(t *testing.T) {
	ctx := context.Background()
	rotation := &stubRotation{}
	service, _ := newComboFixture(t, ctx, rotation)
	seedComboReferences(t, ctx, service)
	created, err := service.Create(ctx, comboDraft(t, "rotating", domain.ComboRoundRobin, 1, "",
		comboRef(t, "openai/gpt-4o", 1), comboRef(t, "openai/gpt-4o-mini", 2)))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cases := []struct {
		name       string
		update     ComboDraft
		wantResets int
	}{
		{
			name:       "a changed list resets",
			update:     comboDraft(t, "rotating", domain.ComboRoundRobin, 1, "", comboRef(t, "openai/gpt-4o", 1)),
			wantResets: 1,
		},
		{
			name:       "an unchanged list does not reset",
			update:     comboDraft(t, "rotating", domain.ComboRoundRobin, 1, "", comboRef(t, "openai/gpt-4o", 1)),
			wantResets: 1,
		},
		{
			name:       "a changed strategy resets",
			update:     comboDraft(t, "rotating", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1)),
			wantResets: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := service.Update(ctx, created.ID(), tc.update); err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if rotation.resets != tc.wantResets {
				t.Fatalf("rotation resets = %d, want %d", rotation.resets, tc.wantResets)
			}
		})
	}
}

// TestComboService_DeleteRefusesWhileAnAliasReferencesIt pins the §7.7 delete
// rule: the alias set must be cleaned first, and the answer is CONFLICT.
func TestComboService_DeleteRefusesWhileAnAliasReferencesIt(t *testing.T) {
	ctx := context.Background()
	service, catalog := newComboFixture(t, ctx, nil)
	seedComboReferences(t, ctx, service)
	created, err := service.Create(ctx, comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1)))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := catalog.service.ReplaceAliases(ctx, []domain.ModelAlias{mustAlias(t, "fast", "daily")}); err != nil {
		t.Fatalf("ReplaceAliases() error = %v", err)
	}

	err = service.Delete(ctx, created.ID())
	if err == nil {
		t.Fatal("Delete() removed a combo an alias still references")
	}
	if !strings.Contains(err.Error(), "still references combo") {
		t.Fatalf("Delete() error = %v, want the referencing alias named", err)
	}
	if code := domain.AsAppError(err).Code; code != "CONFLICT" {
		t.Fatalf("Delete() code = %q, want CONFLICT", code)
	}

	if err := catalog.service.ReplaceAliases(ctx, []domain.ModelAlias{}); err != nil {
		t.Fatalf("ReplaceAliases() error = %v", err)
	}
	if err := service.Delete(ctx, created.ID()); err != nil {
		t.Fatalf("Delete() after clearing the aliases error = %v", err)
	}
	if _, err := service.Get(ctx, created.ID()); !errors.Is(err, domain.ErrComboNotFound) {
		t.Fatalf("Get() after Delete() = %v, want ErrComboNotFound", err)
	}
}
