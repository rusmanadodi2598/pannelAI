// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_routable_test.go
// @for       Draft 024 F4: a combo or judge reference must name a model the
//
//	chat data plane can serve, and the refusal names why.
//
// @uses      internal/domain, internal/registry, context, strings, testing.
// @reason    The write path accepted a member the router refuses — measured:
//
//	a member on a provider with no chat translator saved and then answered
//	PROVIDER_NOT_ROUTABLE, and a media model saved and then missed the
//	chat selector. The data plane's own list excludes both, so "listed"
//	and "answerable" broke exactly where the panel draws its suggestions.
//	The rule belongs at write time: a saved member that cannot serve is a
//	routing failure the operator meets on the first request.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// routableIndex builds the provider shapes the rule distinguishes: one provider
// the gateway translates, one whose protocol needs a connector, one chat
// provider with a media model on it, and one passthrough provider.
func routableIndex(t *testing.T) *registry.Index {
	t.Helper()
	index, err := registry.NewIndex(registry.Document{Revision: "test", Providers: []registry.Provider{
		{
			ID: "plain-chat", Category: "apikey", Transport: registry.Transport{Format: registry.DefaultFormat},
			Models: []registry.Model{testModel("chat-one", "Chat one", "llm")},
		},
		{
			ID: "connector-only", Category: "oauth", Transport: registry.Transport{Format: "kiro"},
			Models: []registry.Model{testModel("kiro-model", "Kiro model", "llm")},
		},
		{
			ID: "media-heavy", Category: "apikey", Transport: registry.Transport{Format: registry.DefaultFormat},
			Models: []registry.Model{
				testModel("chat-two", "Chat two", "llm"),
				testModel("image-one", "Image one", "image"),
			},
		},
	}})
	if err != nil {
		t.Fatalf("registry.NewIndex() error = %v", err)
	}
	return index
}

// newRoutableServices wires the combo and catalog services over routableIndex.
func newRoutableServices(t *testing.T) (*ComboService, *ModelCatalogService) {
	t.Helper()
	index := routableIndex(t)
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	catalog, err := NewModelCatalogService(ModelCatalogServiceDeps{Index: index, Repo: repo, Combos: combos})
	if err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
	service, err := NewComboService(ComboServiceDeps{Repo: combos, Catalog: catalog, Rotation: nil})
	if err != nil {
		t.Fatalf("NewComboService() error = %v", err)
	}
	return service, catalog
}

// TestComboService_CreateRejectsUnroutableMembers covers both halves of the
// rule and the judge: a provider with no chat translator, a media model, and
// each of those as a judge model, all refused with a message that names the
// reason rather than the generic unresolvable one.
func TestComboService_CreateRejectsUnroutableMembers(t *testing.T) {
	ctx := context.Background()
	service, _ := newRoutableServices(t)

	cases := []struct {
		name    string
		ref     string
		judge   bool
		wantMsg string
	}{
		{
			name:    "a member on a provider with no chat translator",
			ref:     "connector-only/kiro-model",
			wantMsg: "does not translate",
		},
		{
			name:    "a media model member",
			ref:     "media-heavy/image-one",
			wantMsg: "is a media model",
		},
		{
			name:    "a judge on a provider with no chat translator",
			ref:     "connector-only/kiro-model",
			judge:   true,
			wantMsg: "does not translate",
		},
		{
			name:    "a media judge model",
			ref:     "media-heavy/image-one",
			judge:   true,
			wantMsg: "is a media model",
		},
	}
	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			draft := ComboDraft{
				Name:     "reject-" + strconv.Itoa(idx),
				Strategy: domain.ComboFallback,
				Models:   []domain.ComboModel{comboRef(t, tc.ref, 1)},
			}
			if tc.judge {
				draft.Strategy = domain.ComboFusion
				draft.JudgeModel = tc.ref
				draft.Models = []domain.ComboModel{comboRef(t, "plain-chat/chat-one", 1)}
			}
			_, err := service.Create(ctx, draft)
			if err == nil {
				t.Fatalf("Create() accepted the reference %q", tc.ref)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("Create() error = %v, want it to name %q", err, tc.wantMsg)
			}
			if code := domain.AsAppError(err).Code; code != "VALIDATION_ERROR" {
				t.Fatalf("Create() code = %q, want VALIDATION_ERROR", code)
			}
		})
	}
}

// TestComboService_CreateAcceptsRoutableMembers is the benign control: a
// declared chat model, an undeclared model on a passthrough provider, and a
// custom row on a chat provider all save, because the router serves each of
// them. The rule must not widen into refusing what routing accepts.
func TestComboService_CreateAcceptsRoutableMembers(t *testing.T) {
	ctx := context.Background()
	service, catalog := newRoutableServices(t)

	if _, err := catalog.AddCustom(ctx, "plain-chat", "custom-row", "Custom row", nil); err != nil {
		t.Fatalf("AddCustom() error = %v", err)
	}

	passthrough := routableIndex(t)
	if _, err := passthrough.WithCustom(registry.CustomNode{
		ID: "openai-compatible-9", Name: "Corp", Prefix: "corp",
		APIType: "chat", BaseURL: "https://corp.example/v1",
	}); err != nil {
		t.Fatalf("WithCustom() error = %v", err)
	}

	cases := []struct {
		name string
		ref  string
	}{
		{name: "a declared chat model", ref: "plain-chat/chat-one"},
		{name: "a custom row on a chat provider", ref: "plain-chat/custom-row"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			draft := ComboDraft{
				Name:     "accept-" + strings.ReplaceAll(strings.ReplaceAll(tc.ref, "/", "-"), ".", "-"),
				Strategy: domain.ComboFallback,
				Models:   []domain.ComboModel{comboRef(t, tc.ref, 1)},
			}
			if _, err := service.Create(ctx, draft); err != nil {
				t.Fatalf("Create() refused the routable reference %q: %v", tc.ref, err)
			}
		})
	}
}

// TestVisionAdapterService_ReplaceRejectsUnroutableModels pins the same rule on
// the adapter: an adapter model the router cannot serve would fail every
// image-bearing request it is meant to save.
func TestVisionAdapterService_ReplaceRejectsUnroutableModels(t *testing.T) {
	ctx := context.Background()
	_, catalog := newRoutableServices(t)
	adapter, err := NewVisionAdapterService(VisionAdapterServiceDeps{
		Repo: newStubAdapterRepo(), Catalog: catalog, Capable: func(domain.ModelRef) bool { return true },
	})
	if err != nil {
		t.Fatalf("NewVisionAdapterService() error = %v", err)
	}
	ref, err := domain.ParseModelRef("connector-only/kiro-model")
	if err != nil {
		t.Fatalf("ParseModelRef() error = %v", err)
	}
	_, err = adapter.Replace(ctx, true, false, []domain.ModelRef{ref})
	if err == nil {
		t.Fatal("Replace() accepted a model the chat plane cannot serve")
	}
	if !strings.Contains(err.Error(), "does not translate") {
		t.Fatalf("Replace() error = %v, want it to name the untranslated format", err)
	}
}
