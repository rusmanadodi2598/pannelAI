// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/vision_augmenter_test.go
// @for       Tests for the data plane's vision augmentation seam: the
//
//	capability decline, where the adapter sits among the request's own
//	candidates, the disabled adapter, the rotation advance, and the
//	advisory rotation failure.
//
// @uses      context, errors, reflect, testing, internal/domain,
//
//	internal/repository.
//
// @reason    SPEC-API-001 §7.8 decides a served request's model order here, so
//
//	every branch — a candidate that reads images, one that cannot, a
//	list holding both, a disabled adapter, the rotation — is pinned
//	against the same in-memory doubles the adapter's own tests use, and
//	the rotation write's advisory contract is stated by a test that
//	makes the store fail.
//
// The candidates are catalog rows the fixture declares by hand rather than
// model ids chosen to trip a name pattern: measured live 2026-09-29 the
// pattern's answer was the thing under test, not a usable lever.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"reflect"
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

// The three models these tests ask about, each chosen for what the catalog can
// be made to say about it:
//
//	seeingModel is a custom row its operator declared vision-capable.
//	blindModel  is a custom row declared an embedding model, so the catalog
//	              refuses it however its name reads.
//	adapterSeed is a registry row the resolver calls vision-capable, which is
//	              what an adapter's own entry has to be to be saved at all.
const (
	seeingModel = "openai/local-see"
	blindModel  = "openai/local-embed"
	adapterSeed = "openai/gpt-4o"
)

// newAugmenter wires the seam over the adapter fixture and seeds the two custom
// rows the tests aim their requests at. The predicate is permissive because a
// candidate the catalog lists is never judged by it — that is the rule under test.
func newAugmenter(t *testing.T, ctx context.Context, rotation repository.VisionRotationStore) (*VisionAugmenter, *VisionAdapterService) {
	t.Helper()
	service, _, catalog := newAdapterFixture(t, ctx, acceptAll)
	if err := catalog.repo.AddCustom(ctx,
		mustCustomModel(t, "openai", "local-see", "Local See", "vision")); err != nil {
		t.Fatalf("seeding a vision-declaring custom model: %v", err)
	}
	augmenter, err := NewVisionAugmenter(VisionAugmenterDeps{Adapter: service, Rotation: rotation})
	if err != nil {
		t.Fatalf("NewVisionAugmenter() error = %v", err)
	}
	return augmenter, service
}

// enabledAdapter seeds the fixture's configuration: one model, enabled, round
// robin, validated through the catalog.
func enabledAdapter(t *testing.T, ctx context.Context, service *VisionAdapterService) {
	t.Helper()
	if _, err := service.Replace(ctx, true, true, []domain.ModelRef{visionRef(t, adapterSeed)}); err != nil {
		t.Fatalf("seeding the adapter configuration: %v", err)
	}
}

// TestVisionAugmenter_DeclinesForACapableModel pins the first §7.8 question: a
// request whose candidate reads images itself never receives augmentation, the
// adapter is not consulted, and its rotation is not spent on a request that did
// not need it.
func TestVisionAugmenter_DeclinesForACapableModel(t *testing.T) {
	ctx := context.Background()
	rotation := &stubVisionRotation{}
	augmenter, service := newAugmenter(t, ctx, rotation)
	enabledAdapter(t, ctx, service)

	refs, adapted, err := augmenter.Augment(ctx, []string{seeingModel})
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if len(adapted) != 0 {
		t.Fatalf("Augment() adapted = %v, want none for a vision-capable candidate", adapted)
	}
	if !reflect.DeepEqual(refs, []string{seeingModel}) {
		t.Fatalf("Augment() refs = %v, want the request's own order", refs)
	}
	if len(rotation.saved) != 0 {
		t.Fatalf("rotation saves = %d, want 0: a request that needed no adapter spends no rotation", len(rotation.saved))
	}
}

// TestVisionAugmenter_DeclinesWhenTheAdapterIsDisabled pins the default: a
// fresh install's disabled adapter must answer "no augmentation" rather than
// routing image requests anywhere else.
func TestVisionAugmenter_DeclinesWhenTheAdapterIsDisabled(t *testing.T) {
	ctx := context.Background()
	augmenter, _ := newAugmenter(t, ctx, nil)

	refs, adapted, err := augmenter.Augment(ctx, []string{blindModel})
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if len(adapted) != 0 || !reflect.DeepEqual(refs, []string{blindModel}) {
		t.Fatalf("Augment() = (%v, %v), want no augmentation while the adapter is disabled", refs, adapted)
	}
}

// TestVisionAugmenter_BlindCandidateIsPutBehindTheAdapter pins the case the
// adapter exists for: nothing the request can be served by reads images, so the
// adapter goes first — the only position that can save it, because a blind
// candidate that answers 200 ends the walk before anything else is tried.
func TestVisionAugmenter_BlindCandidateIsPutBehindTheAdapter(t *testing.T) {
	ctx := context.Background()
	rotation := &stubVisionRotation{}
	augmenter, service := newAugmenter(t, ctx, rotation)
	enabledAdapter(t, ctx, service)

	refs, adapted, err := augmenter.Augment(ctx, []string{blindModel})
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if !reflect.DeepEqual(refs, []string{adapterSeed, blindModel}) {
		t.Fatalf("Augment() refs = %v, want the adapter before the blind candidate", refs)
	}
	if !reflect.DeepEqual(adapted, []string{adapterSeed}) {
		t.Fatalf("Augment() adapted = %v, want the adapter's own model reported as the adapter's", adapted)
	}
	if len(rotation.saved) != 1 {
		t.Fatalf("rotation saves = %d, want one advance per served augmentation", len(rotation.saved))
	}
}

// TestVisionAugmenter_CapableCandidateOutranksTheAdapter is the measured bug: a
// combo whose own member reads images was displaced by an adapter that does not,
// because only the leading candidate was judged and the adapter was always
// inserted in front of the whole list.
func TestVisionAugmenter_CapableCandidateOutranksTheAdapter(t *testing.T) {
	ctx := context.Background()
	augmenter, service := newAugmenter(t, ctx, nil)
	enabledAdapter(t, ctx, service)

	refs, adapted, err := augmenter.Augment(ctx, []string{blindModel, seeingModel})
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if !reflect.DeepEqual(refs, []string{seeingModel, adapterSeed, blindModel}) {
		t.Fatalf("Augment() refs = %v, want capable first, adapter next, blind last", refs)
	}
	if !reflect.DeepEqual(adapted, []string{adapterSeed}) {
		t.Fatalf("Augment() adapted = %v, want only the adapter's own model reported", adapted)
	}
}

// TestVisionAugmenter_AdvisoryRotationFailureDoesNotBlockServing pins the
// rotation store's contract: a store that cannot answer costs the rotation,
// never the request — the augmentation still applies.
func TestVisionAugmenter_AdvisoryRotationFailureDoesNotBlockServing(t *testing.T) {
	ctx := context.Background()
	augmenter, service := newAugmenter(t, ctx, failingVisionRotation{})
	enabledAdapter(t, ctx, service)

	refs, adapted, err := augmenter.Augment(ctx, []string{blindModel})
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if len(adapted) == 0 || len(refs) != 2 {
		t.Fatalf("Augment() = (%v, %v), want the order despite the store failing", refs, adapted)
	}
}
