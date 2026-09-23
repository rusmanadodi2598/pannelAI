// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_selfref_test.go
// @for       The self-reference and alias-target halves of the combo write
//
//	rules: a combo cannot list itself, and an alias member is judged by
//	whatever its target is.
//
// @uses      internal/domain, context, strings, testing.
// @reason    A self-reference is the one cycle a write can produce without
//
//	touching the database, and an alias hop is how a media or
//	untranslatable target would otherwise reach a saved member two writes
//	after the rule that refuses the direct spelling. Both are pinned
//	here, separated from the servability table at the AGENTS.md §1.1
//	line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestComboService_CreateRefusesASelfReference pins the write-time half of the
// cycle rule: a combo that lists itself is refused, so the runtime guard is a
// backstop for stored data rather than the only defence.
func TestComboService_CreateRefusesASelfReference(t *testing.T) {
	ctx := context.Background()
	service, _ := servableServices(t)
	draft := ComboDraft{
		Name:     "loop",
		Strategy: domain.ComboFallback,
		Models:   []domain.ComboModel{comboRef(t, "loop", 1)},
	}
	_, err := service.Create(ctx, draft)
	if err == nil {
		t.Fatal("Create() accepted a combo that lists itself")
	}
	if !strings.Contains(err.Error(), "itself") {
		t.Fatalf("Create() error = %v, want it to name the self-reference", err)
	}
}

// TestComboService_UpdateRefusesASelfReference pins the same rule on the patch
// route, which is where an existing combo could acquire the loop.
func TestComboService_UpdateRefusesASelfReference(t *testing.T) {
	ctx := context.Background()
	service, _ := servableServices(t)
	created, err := service.Create(ctx, ComboDraft{
		Name:     "loop-patch",
		Strategy: domain.ComboFallback,
		Models:   []domain.ComboModel{comboRef(t, "declared-only/known", 1)},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, err = service.Update(ctx, created.ID(), ComboDraft{
		Name:     "loop-patch",
		Strategy: domain.ComboFallback,
		Models:   []domain.ComboModel{comboRef(t, "loop-patch", 1)},
	})
	if err == nil {
		t.Fatal("Update() accepted a combo that lists itself")
	}
	if !strings.Contains(err.Error(), "itself") {
		t.Fatalf("Update() error = %v, want it to name the self-reference", err)
	}
}

// TestComboService_CreateJudgesAnAliasTarget pins the alias hop: a member that
// names an alias is served by whatever the alias targets, so an alias to a media
// model or to an untranslatable provider is refused exactly like a direct
// reference to it would be.
func TestComboService_CreateJudgesAnAliasTarget(t *testing.T) {
	ctx := context.Background()
	service, catalog := servableServices(t)
	if err := catalog.ReplaceAliases(ctx, []domain.ModelAlias{
		mustAlias(t, "pic", "media-provider/image-one"),
		mustAlias(t, "good", "declared-only/known"),
	}); err != nil {
		t.Fatalf("ReplaceAliases() error = %v", err)
	}

	cases := []struct {
		name    string
		ref     string
		wantMsg string
	}{
		{name: "an alias to a chat model saves", ref: "good"},
		{name: "an alias to a media model is refused with its kind", ref: "pic", wantMsg: "is a media model"},
	}
	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			draft := ComboDraft{
				Name:     "alias-ref-" + string(rune('a'+idx)),
				Strategy: domain.ComboFallback,
				Models:   []domain.ComboModel{comboRef(t, tc.ref, 1)},
			}
			_, err := service.Create(ctx, draft)
			if tc.wantMsg == "" {
				if err != nil {
					t.Fatalf("Create(%q) error = %v, want it accepted", tc.ref, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Create(%q) accepted an alias whose target the router cannot serve", tc.ref)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("Create(%q) error = %v, want it to name %q", tc.ref, err, tc.wantMsg)
			}
		})
	}
}
