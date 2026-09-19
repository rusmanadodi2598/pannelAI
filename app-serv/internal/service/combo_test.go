// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_test.go
// @for       Table-driven tests for the combo lifecycle and its ref validation (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/domain, internal/repository, context, errors, strings,
// @reason    §7.7 makes write time the only moment a combo's references can be
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

// TestComboService_Create pins the create path: every reference shape §7.7
// allows is accepted, and an unresolvable one is a VALIDATION_ERROR at write
// time.
func TestComboService_Create(t *testing.T) {
	ctx := context.Background()
	judge := "openai/gpt-4o"
	cases := []struct {
		name    string
		draft   ComboDraft
		wantErr string
	}{
		{name: "a model reference", draft: comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1))},
		{name: "a custom model reference", draft: comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "openai/local-embed", 1))},
		{name: "a reference to another combo", draft: comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "seeded-combo", 1))},
		{name: "a reference to an alias", draft: comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "fast", 1))},
		{name: "an alias that points at a combo", draft: comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "daily-alias", 1))},
		{name: "round_robin with a sticky limit", draft: comboDraft(t, "rotating", domain.ComboRoundRobin, 3, "", comboRef(t, "openai/gpt-4o", 1))},
		{name: "fusion with a judge model", draft: comboDraft(t, "panel", domain.ComboFusion, 0, judge, comboRef(t, "openai/gpt-4o-mini", 1))},
		{
			name: "a mixed list of a model, a combo, and an alias",
			draft: comboDraft(t, "mixed", domain.ComboFallback, 0, "",
				comboRef(t, "openai/gpt-4o", 1), comboRef(t, "seeded-combo", 2), comboRef(t, "fast", 3)),
		},
		{
			name:    "an unresolvable reference",
			draft:   comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "openai/ghost", 1)),
			wantErr: "does not resolve",
		},
		{
			name:    "a reference to a disabled model",
			draft:   comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "anthropic/claude-3", 1)),
			wantErr: "does not resolve",
		},
		{
			name:    "an unresolvable judge model",
			draft:   comboDraft(t, "panel", domain.ComboFusion, 0, "openai/ghost", comboRef(t, "openai/gpt-4o", 1)),
			wantErr: "does not resolve",
		},
		{
			name:    "a fusion combo without a judge",
			draft:   comboDraft(t, "panel", domain.ComboFusion, 0, "", comboRef(t, "openai/gpt-4o", 1)),
			wantErr: "requires judge_model",
		},
		{
			name:    "a round_robin combo without a sticky limit",
			draft:   comboDraft(t, "rotating", domain.ComboRoundRobin, 0, "", comboRef(t, "openai/gpt-4o", 1)),
			wantErr: "sticky_limit",
		},
		{
			name:    "a fallback combo with a judge model",
			draft:   comboDraft(t, "daily", domain.ComboFallback, 0, judge, comboRef(t, "openai/gpt-4o", 1)),
			wantErr: "does not accept judge_model",
		},
		{
			name:    "a combo with no models",
			draft:   ComboDraft{Name: "empty", Strategy: domain.ComboFallback},
			wantErr: "at least one model",
		},
		{
			name:    "an invalid name",
			draft:   comboDraft(t, "bad name", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1)),
			wantErr: "may only contain",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := newComboFixture(t, ctx, nil)
			seedComboReferences(t, ctx, service)
			combo, err := service.Create(ctx, tc.draft)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Create() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Create() error = %v, want %q", err, tc.wantErr)
				}
				if code := domain.AsAppError(err).Code; code != "VALIDATION_ERROR" {
					t.Fatalf("Create() code = %q, want VALIDATION_ERROR", code)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if !strings.HasPrefix(combo.ID(), domain.ComboIDPrefix) {
				t.Fatalf("Create() id = %q, want the %s prefix", combo.ID(), domain.ComboIDPrefix)
			}
		})
	}
}

// TestComboService_CreateRejectsADuplicateName pins the uniqueness the index
// enforces, through the sentinel the service maps to CONFLICT.
func TestComboService_CreateRejectsADuplicateName(t *testing.T) {
	ctx := context.Background()
	service, _ := newComboFixture(t, ctx, nil)
	seedComboReferences(t, ctx, service)
	input := comboDraft(t, "daily", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1))

	if _, err := service.Create(ctx, input); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, err := service.Create(ctx, input)
	if err == nil {
		t.Fatal("Create() accepted a duplicate name")
	}
	if !errors.Is(err, domain.ErrComboExists) {
		t.Fatalf("Create() error = %v, want ErrComboExists", err)
	}
	if code := domain.AsAppError(err).Code; code != "CONFLICT" {
		t.Fatalf("Create() code = %q, want CONFLICT", code)
	}
}
