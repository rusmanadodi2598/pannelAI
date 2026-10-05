// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_detail_test.go
// @for       The provider detail read: the entry, its roll-up, and the custom
//
//	rows the §7.14 level union is computed from.
//
// @uses      internal/domain, internal/registry, context, errors, testing.
// @reason    SPEC-API-001 §7.4 serves one provider's detail, and §7.14 makes
//
//	that body the union of the levels its models accept. A custom node has
//	no registry models, so the read this file covers is what decides
//	whether its screen carries a reasoning picker at all; and a read that
//	failed must not be reported as "declares none", which is the
//	distinction the second and third cases exist to hold.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stubCustomModels is a CustomModelLister that answers a fixed value or a fixed
// error, and records the provider id it was asked for so a test can assert the
// read is the provider's own.
type stubCustomModels struct {
	rows   []domain.CustomModel
	err    error
	calls  int
	lastID string
}

func (s *stubCustomModels) Custom(_ context.Context, providerID string) ([]domain.CustomModel, error) {
	s.calls++
	s.lastID = providerID
	return s.rows, s.err
}

// detailRow builds one declared row for the stub to answer. The id is the
// fixture's own, `mdl_<modelID>`, so the assertions name a row the other
// catalog tests would name the same way.
func detailRow(t *testing.T, providerID, modelID string) domain.CustomModel {
	t.Helper()
	return mustCustomModel(t, providerID, modelID, modelID, "vision")
}

// newDetailService wires the detail read over a one-entry index and an optional
// lister. A nil lister is normalized to an unset interface rather than assigned
// as a typed nil, which is the trap newProviderServiceWithSource documents for
// the model source.
func newDetailService(t *testing.T, entry registry.Provider, lister CustomModelLister) *ProviderService {
	t.Helper()
	deps := ProviderServiceDeps{Index: readinessProviderIndex{entries: []registry.Provider{entry}}}
	if lister != nil {
		deps.Custom = lister
	}
	svc, err := NewProviderService(deps)
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	return svc
}

// TestProviderService_Detail_CarriesTheDeclaredRows pins the read the level
// union's second half comes from: the rows are the provider's own, they travel
// on the row the handler projects, and a service with no lister answers none
// rather than failing.
func TestProviderService_Detail_CarriesTheDeclaredRows(t *testing.T) {
	node := customNodeEntry("openai-compatible-01TEST")
	rows := []domain.CustomModel{
		detailRow(t, node.ID, "claude-opus-4-8"),
		detailRow(t, node.ID, "mystery-1"),
	}
	lister := &stubCustomModels{rows: rows}
	svc := newDetailService(t, node, lister)

	got, err := svc.Detail(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("Detail(%q) error = %v, want nil", node.ID, err)
	}
	if len(got.CustomModels) != 2 || got.CustomModels[0].ID() != "mdl_claude-opus-4-8" {
		t.Fatalf("Detail(%q).CustomModels = %v, want the two declared rows", node.ID, got.CustomModels)
	}
	if lister.calls != 1 || lister.lastID != node.ID {
		t.Fatalf("the lister was called %d times for %q, want once for %q", lister.calls, lister.lastID, node.ID)
	}

	// A deployment that declares no custom model still renders a detail: the
	// union then falls back to the registry's own models.
	withoutLister := newDetailService(t, node, nil)
	got, err = withoutLister.Detail(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("Detail() with no lister error = %v, want nil", err)
	}
	if got.CustomModels != nil {
		t.Fatalf("Detail() with no lister answered %v, want no rows", got.CustomModels)
	}
}

// TestProviderService_Detail_AFailedRowReadIsNotAnEmptyAnswer pins the
// distinction the union depends on: "this provider declares no custom model" is
// a fact the picker may act on, and a read that failed is not that fact.
func TestProviderService_Detail_AFailedRowReadIsNotAnEmptyAnswer(t *testing.T) {
	node := customNodeEntry("openai-compatible-01TEST")
	lister := &stubCustomModels{err: errors.New("the store is unreachable")}
	svc := newDetailService(t, node, lister)

	if _, err := svc.Detail(context.Background(), node.ID); err == nil {
		t.Fatal("Detail() reported no error while the declared rows could not be read")
	}
}

// TestProviderService_Detail_UnknownProviderSkipsTheRowRead pins that the
// not-found answer comes first: a provider that is not in the registry has no
// declared rows to read, so the store is never consulted for one.
func TestProviderService_Detail_UnknownProviderSkipsTheRowRead(t *testing.T) {
	node := customNodeEntry("openai-compatible-01TEST")
	lister := &stubCustomModels{}
	svc := newDetailService(t, node, lister)

	_, err := svc.Detail(context.Background(), "no-such-provider")
	if domain.AsAppError(err).Code != "NOT_FOUND" {
		t.Fatalf("Detail(unknown) error = %v, want NOT_FOUND", err)
	}
	if lister.calls != 0 {
		t.Fatalf("the lister was called %d times for an unknown provider, want 0", lister.calls)
	}
}
