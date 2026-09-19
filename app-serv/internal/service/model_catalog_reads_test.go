// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_reads_test.go
// @for       The write-path tests for the catalog: custom models, the alias set, the disabled set. (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/domain, context, strings, testing, time.
// @reason    The read tests and the write tests need different setups; AGENTS.md §1.1 caps a file at 250 lines, so the write paths and the resolution tests moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestModelCatalogService_ReplaceAliases pins the whole-set replacement and its
// validation: a target must exist as a model or a combo, and a bad row refuses
// the whole write.
func TestModelCatalogService_ReplaceAliases(t *testing.T) {
	ctx := context.Background()
	fixture := newCatalogFixture(t, ctx)
	if err := fixture.combos.Create(ctx, mustCombo(t, "cmb_one", "fallback-combo")); err != nil {
		t.Fatalf("seeding a combo: %v", err)
	}

	cases := []struct {
		name      string
		aliases   []domain.ModelAlias
		wantErr   string
		wantCount int
	}{
		{
			name: "a target model and a target combo",
			aliases: []domain.ModelAlias{
				mustAlias(t, "fast", "openai/gpt-4o-mini"),
				mustAlias(t, "daily", "fallback-combo"),
			},
			wantCount: 2,
		},
		{
			name:      "an empty set clears every alias",
			aliases:   []domain.ModelAlias{},
			wantCount: 0,
		},
		{
			name:      "a target that does not exist",
			aliases:   []domain.ModelAlias{mustAlias(t, "ghost", "openai/ghost")},
			wantErr:   "unknown model or combo",
			wantCount: 0,
		},
		{
			name:      "a disabled model is not a valid target",
			aliases:   []domain.ModelAlias{mustAlias(t, "hidden", "anthropic/claude-3")},
			wantErr:   "unknown model or combo",
			wantCount: 0,
		},
		{
			name:      "an alias may not shadow a combo name",
			aliases:   []domain.ModelAlias{mustAlias(t, "fallback-combo", "openai/gpt-4o")},
			wantErr:   "already a combo name",
			wantCount: 0,
		},
		{
			name: "one bad row refuses the whole set",
			aliases: []domain.ModelAlias{
				mustAlias(t, "ok", "openai/gpt-4o"),
				mustAlias(t, "bad", "openai/ghost"),
			},
			wantErr:   "unknown model or combo",
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fixture.service.ReplaceAliases(ctx, tc.aliases)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("ReplaceAliases() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ReplaceAliases() error = %v, want %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("ReplaceAliases() error = %v", err)
			}
			stored, err := fixture.service.Aliases(ctx)
			if err != nil {
				t.Fatalf("Aliases() error = %v", err)
			}
			if tc.wantErr != "" {
				// A refused write leaves the previous set in place; the first
				// case stored two, and only the successful cases change the
				// count afterwards.
				return
			}
			if len(stored) != tc.wantCount {
				t.Fatalf("Aliases() = %d entries, want %d", len(stored), tc.wantCount)
			}
		})
	}
}

// TestModelCatalogService_ReplaceDisabled pins the disabled set replacement and
// the rule that a pair must name a catalog model — including one that is already
// disabled, so re-submitting the same set is idempotent.
func TestModelCatalogService_ReplaceDisabled(t *testing.T) {
	ctx := context.Background()
	fixture := newCatalogFixture(t, ctx)
	openai, err := domain.NewModelRef("openai", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("NewModelRef() error = %v", err)
	}
	claude, err := domain.NewModelRef("anthropic", "claude-3")
	if err != nil {
		t.Fatalf("NewModelRef() error = %v", err)
	}
	unknown, err := domain.NewModelRef("openai", "ghost")
	if err != nil {
		t.Fatalf("NewModelRef() error = %v", err)
	}
	zero := domain.ModelRef{}

	cases := []struct {
		name      string
		refs      []domain.ModelRef
		wantErr   string
		wantCount int
	}{
		{name: "adding one pair", refs: []domain.ModelRef{claude, openai}, wantCount: 2},
		{
			name: "re-submitting an already disabled pair stays valid",
			refs: []domain.ModelRef{claude, openai}, wantCount: 2,
		},
		{name: "clearing the set", refs: []domain.ModelRef{}, wantCount: 0},
		{name: "an unknown model", refs: []domain.ModelRef{unknown}, wantErr: "unknown model", wantCount: 0},
		{name: "a zero reference", refs: []domain.ModelRef{zero}, wantErr: "required", wantCount: 0},
		{
			name:      "one bad row refuses the whole set",
			refs:      []domain.ModelRef{openai, unknown},
			wantErr:   "unknown model",
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fixture.service.ReplaceDisabled(ctx, tc.refs)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("ReplaceDisabled() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ReplaceDisabled() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReplaceDisabled() error = %v", err)
			}
			stored, err := fixture.service.Disabled(ctx)
			if err != nil {
				t.Fatalf("Disabled() error = %v", err)
			}
			if len(stored) != tc.wantCount {
				t.Fatalf("Disabled() = %d entries, want %d", len(stored), tc.wantCount)
			}
		})
	}
}
