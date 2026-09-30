// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_vision_test.go
// @for       Who answers "can this model read an image" — the catalog the
//
//	operator wrote, or the predicate the composition root injected.
//
// @uses      context, testing, internal/domain.
// @reason    Measured live on 2026-09-29: the model configured as the vision
//
//	adapter answered a solid-red image as "gray" and said so in its own
//	reasoning, while a pass-through model no table knew about answered it
//	correctly. Both answers came from a name pattern that could not see the
//	operator's own declarations. These tests pin the catalog as the authority
//	in both directions and keep the predicate where it still decides: an id
//	no catalog row speaks for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestModelCatalogVisionCapable(t *testing.T) {
	ctx := context.Background()
	catalog := newCatalogFixture(t, ctx)
	if err := catalog.repo.AddCustom(ctx,
		mustCustomModel(t, "openai", "seeing-local", "Seeing Local", "vision")); err != nil {
		t.Fatalf("seeding a vision-declaring custom model: %v", err)
	}
	// A custom row over the same (provider, model) pair replaces the registry row,
	// which is exactly the shape a custom node's model has: a row that says nothing
	// about images. The resolver still calls this id vision-capable, so this is the
	// case that decides whether a silent row is a denial.
	if err := catalog.repo.AddCustom(ctx,
		mustCustomModel(t, "openai", "gpt-4o-mini", "Local Mini", "tools")); err != nil {
		t.Fatalf("seeding a vision-silent custom row: %v", err)
	}

	cases := []struct {
		name         string
		ref          string
		wantCapable  bool
		wantAnswered bool
	}{
		{
			name: "a registry row is answered by the resolver", ref: "openai/gpt-4o",
			wantCapable: true, wantAnswered: true,
		},
		{
			// Additive, not vetoing: a row that does not mention vision is not the
			// operator denying it. This is the case that routed a reference-declared
			// vision model to the adapter, because a custom node's row mentions
			// nothing about images.
			name: "a custom row that omits vision does not deny the resolver",
			ref:  "openai/gpt-4o-mini", wantCapable: true, wantAnswered: true,
		},
		{
			// The lever no pattern can supply: a model nobody registered, declared
			// capable by the operator who watched it read an image.
			name: "a custom row that declares vision", ref: "openai/seeing-local",
			wantCapable: true, wantAnswered: true,
		},
		{
			// The row declares an embedding model and no pattern claims vision for it
			// either, so this is the refusal §7.8's guard is written to produce.
			name: "an embedding row is refused", ref: "openai/local-embed",
			wantCapable: false, wantAnswered: true,
		},
		{
			name: "a model the catalog does not list is left for the caller",
			ref:  "opencode/space-bunny-free", wantAnswered: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capable, answered, err := catalog.service.VisionCapable(ctx, visionRef(t, tc.ref))
			if err != nil {
				t.Fatalf("VisionCapable() error = %v", err)
			}
			if answered != tc.wantAnswered {
				t.Fatalf("VisionCapable() answered = %t, want %t", answered, tc.wantAnswered)
			}
			if capable != tc.wantCapable {
				t.Fatalf("VisionCapable(%q) = %t, want %t", tc.ref, capable, tc.wantCapable)
			}
		})
	}
}

// TestVisionAdapterServiceVisionCapableFallsBackToThePredicate pins the one case
// the injected predicate still decides: a pass-through provider's id that no
// catalog row carries.
func TestVisionAdapterServiceVisionCapableFallsBackToThePredicate(t *testing.T) {
	ctx := context.Background()
	judged := make([]string, 0, 2)
	service, _, _ := newAdapterFixture(t, ctx, func(ref domain.ModelRef) bool {
		judged = append(judged, ref.String())
		return ref.ModelID() == "sees"
	})

	capable, err := service.VisionCapable(ctx, visionRef(t, "opencode/sees"))
	if err != nil {
		t.Fatalf("VisionCapable() error = %v", err)
	}
	if !capable {
		t.Fatal("an unlisted model the predicate accepts was refused")
	}

	blind, err := service.VisionCapable(ctx, visionRef(t, "opencode/blind"))
	if err != nil {
		t.Fatalf("VisionCapable() error = %v", err)
	}
	if blind {
		t.Fatal("an unlisted model the predicate rejects was accepted")
	}
	if len(judged) != 2 {
		t.Fatalf("the predicate was consulted %d times, want once per unlisted model", len(judged))
	}
}
