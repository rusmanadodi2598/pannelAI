// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/vision_augmenter_test.go
// @for       Tests for the data plane's vision augmentation seam: the
//
//	capability decline, the disabled adapter, the enabled order with
//	its rotation advance, and the advisory rotation failure.
//
// @uses      testing, context, internal/domain.
// @reason    SPEC-API-001 §7.8 decides a served request's model order here, so
//
//	every branch — capable model, disabled adapter, rotation — is
//	pinned against the same in-memory doubles the adapter's own
//	tests use, and the rotation write's advisory contract is stated
//	by a test that makes the store fail.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// stubVisionRotation records the states written to it and answers with one
// stored state, so a test can pin the advance across two asks.
type stubVisionRotation struct {
	state domain.RotationState
	saved []domain.RotationState
}

func (r *stubVisionRotation) Get(context.Context) (domain.RotationState, error) {
	return r.state, nil
}

func (r *stubVisionRotation) Save(_ context.Context, state domain.RotationState) error {
	r.saved = append(r.saved, state)
	r.state = state
	return nil
}

// failingVisionRotation answers every call with an error.
type failingVisionRotation struct{}

func (failingVisionRotation) Get(context.Context) (domain.RotationState, error) {
	return domain.RotationState{}, errors.New("redis is down")
}

func (failingVisionRotation) Save(context.Context, domain.RotationState) error {
	return errors.New("redis is down")
}

// enabledAdapter seeds the fixture's configuration: one model, enabled, round
// robin, validated through the accepting predicate.
func enabledAdapter(t *testing.T, ctx context.Context, service *VisionAdapterService) {
	t.Helper()
	_, err := service.Replace(ctx, true, true, []domain.ModelRef{visionRef(t, "openai/gpt-4o-mini")})
	if err != nil {
		t.Fatalf("seeding the adapter configuration: %v", err)
	}
}

// onlyMini judges every model capable except gpt-4o, so a test can aim an
// image request at a blind model while the adapter's own model stays valid.
func onlyMini(ref domain.ModelRef) bool { return ref.ModelID() != "gpt-4o" }

// newAugmenter wires the seam over the adapter fixture with the predicate
// above, so the requested model is judged blind and the adapter's is not.
func newAugmenter(t *testing.T, ctx context.Context, rotation repository.VisionRotationStore) (*VisionAugmenter, *VisionAdapterService) {
	t.Helper()
	service, _, _ := newAdapterFixture(t, ctx, onlyMini)
	augmenter, err := NewVisionAugmenter(VisionAugmenterDeps{
		Adapter: service, Capable: onlyMini, Rotation: rotation,
	})
	if err != nil {
		t.Fatalf("NewVisionAugmenter() error = %v", err)
	}
	return augmenter, service
}

// TestVisionAugmenter_DeclinesForACapableModel pins the first §7.8 question: a
// model that reads images itself never receives augmentation, and the adapter
// is not even consulted.
func TestVisionAugmenter_DeclinesForACapableModel(t *testing.T) {
	ctx := context.Background()
	augmenter, _ := newAugmenter(t, ctx, nil)

	refs, applies, err := augmenter.Augment(ctx, "openai", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if applies || len(refs) != 0 {
		t.Fatalf("Augment() = (%v, %d refs), want no augmentation for a vision-capable model", applies, len(refs))
	}
}

// TestVisionAugmenter_DeclinesWhenTheAdapterIsDisabled pins the default: a
// fresh install's disabled adapter must answer "no augmentation" rather than
// routing image requests anywhere else.
func TestVisionAugmenter_DeclinesWhenTheAdapterIsDisabled(t *testing.T) {
	ctx := context.Background()
	augmenter, _ := newAugmenter(t, ctx, nil)

	refs, applies, err := augmenter.Augment(ctx, "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if applies || len(refs) != 0 {
		t.Fatalf("Augment() = (%v, %d refs), want no augmentation while the adapter is disabled", applies, len(refs))
	}
}

// TestVisionAugmenter_EnabledAdapterReturnsTheOrderAndAdvancesTheRotation pins
// the served path: an enabled adapter's order is handed over by reference, and
// the state the order produced is persisted for the next image-bearing request.
func TestVisionAugmenter_EnabledAdapterReturnsTheOrderAndAdvancesTheRotation(t *testing.T) {
	ctx := context.Background()
	rotation := &stubVisionRotation{}
	augmenter, adapter := newAugmenter(t, ctx, rotation)
	enabledAdapter(t, ctx, adapter)

	refs, applies, err := augmenter.Augment(ctx, "openai", "gpt-4o")
	if err != nil || !applies || len(refs) != 1 {
		t.Fatalf("Augment() = (%v, %v, %d refs), want the adapter's order", err, applies, len(refs))
	}
	if refs[0] != "openai/gpt-4o-mini" {
		t.Fatalf("Augment() refs = %v, want the configured adapter model", refs)
	}
	if len(rotation.saved) != 1 {
		t.Fatalf("rotation saves = %d, want one advance per served augmentation", len(rotation.saved))
	}
}

// TestVisionAugmenter_AdvisoryRotationFailureDoesNotBlockServing pins the
// rotation store's contract: a store that cannot answer costs the rotation,
// never the request — the augmentation still applies.
func TestVisionAugmenter_AdvisoryRotationFailureDoesNotBlockServing(t *testing.T) {
	ctx := context.Background()
	augmenter, adapter := newAugmenter(t, ctx, failingVisionRotation{})
	enabledAdapter(t, ctx, adapter)

	refs, applies, err := augmenter.Augment(ctx, "openai", "gpt-4o")
	if err != nil || !applies || len(refs) != 1 {
		t.Fatalf("Augment() = (%v, %v, %d refs), want the order despite the store failing", err, applies, len(refs))
	}
}
