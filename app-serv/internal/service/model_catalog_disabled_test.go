// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_disabled_test.go
// @for       The two-sided canonical comparison the disabled set applies, so one
//
//	model is hidden whichever of its names the pair was stored under.
//
// @uses      internal/domain, context, testing.
// @reason    Draft 024 F2 after review: a pair stored under a node prefix must
//
//	hide the node-id spelling too, and a pair stored under the id must
//	hide the alias spelling, because the set is a judgement about the
//	model rather than about one of its names. Separated from the
//	canonical-form table at the AGENTS.md §1.1 line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestModelCatalogService_DisabledHidesEverySpelling pins the two-sided
// canonical comparison: a pair stored under a node prefix hides the node's id
// spelling, and a pair stored under the id hides the alias spelling, because the
// set is a judgement about the model rather than about one of its names.
func TestModelCatalogService_DisabledHidesEverySpelling(t *testing.T) {
	ctx := context.Background()
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	service, err := NewModelCatalogService(ModelCatalogServiceDeps{
		Index: canonicalNodeIndex(t), Repo: repo, Combos: combos,
	})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	cases := []struct {
		name     string
		stored   string
		asked    []string
		notAsked string
	}{
		{
			name:   "a pair stored under the node prefix hides the node-id spelling",
			stored: "corp/corp-chat",
			asked:  []string{"corp/corp-chat", "openai-compatible-1/corp-chat"},
		},
		{
			name:   "a pair stored under the id hides the prefix spelling",
			stored: "openai-compatible-1/corp-chat",
			asked:  []string{"corp/corp-chat", "openai-compatible-1/corp-chat"},
		},
		{
			name:   "a pair stored under the registry id hides the alias spelling",
			stored: "kserve/glm-4.7",
			asked:  []string{"kserve/glm-4.7", "ks/glm-4.7"},
		},
		{
			name:     "an unrelated model stays visible",
			stored:   "kserve/glm-4.7",
			asked:    []string{},
			notAsked: "corp/corp-chat",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored, err := domain.ParseModelRef(tc.stored)
			if err != nil {
				t.Fatalf("ParseModelRef(%q) error = %v", tc.stored, err)
			}
			if err := service.ReplaceDisabled(ctx, []domain.ModelRef{stored}); err != nil {
				t.Fatalf("ReplaceDisabled() error = %v", err)
			}
			for _, raw := range tc.asked {
				ref, err := domain.ParseModelRef(raw)
				if err != nil {
					t.Fatalf("ParseModelRef(%q) error = %v", raw, err)
				}
				hidden, err := service.isDisabled(ctx, ref)
				if err != nil {
					t.Fatalf("isDisabled(%q) error = %v", raw, err)
				}
				if !hidden {
					t.Fatalf("isDisabled(%q) = false, want the stored %q to hide it", raw, tc.stored)
				}
			}
			if tc.notAsked != "" {
				ref, err := domain.ParseModelRef(tc.notAsked)
				if err != nil {
					t.Fatalf("ParseModelRef(%q) error = %v", tc.notAsked, err)
				}
				hidden, err := service.isDisabled(ctx, ref)
				if err != nil {
					t.Fatalf("isDisabled(%q) error = %v", tc.notAsked, err)
				}
				if hidden {
					t.Fatalf("isDisabled(%q) = true, want it visible beside %q", tc.notAsked, tc.stored)
				}
			}
		})
	}
}
