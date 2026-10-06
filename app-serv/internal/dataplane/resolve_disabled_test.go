// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/resolve_disabled_test.go
// @for       The routing refusal a disabled pair must produce, and the fail-open on a failed read.
// @uses      context, errors, testing, internal/domain
// @reason    SPEC-API-001 §7.6 says a disabled model is hidden from routing, and the listing already hid it: without this the operator's switch moved a model off the menu while leaving the door open, so a client naming it was still served and still billed.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestResolver_DisabledRefusesRouting(t *testing.T) {
	cases := []struct {
		name     string
		lookup   fakeLookup
		provider string
		model    string
		wantErr  bool
	}{
		{
			name:     "a declared model the operator disabled is refused",
			lookup:   fakeLookup{disabled: []domain.ModelRef{ref(t, "lit", "lit-chat")}},
			provider: "lit", model: "lit-chat", wantErr: true,
		},
		{
			name:     "disabling one model leaves its sibling routable",
			lookup:   fakeLookup{disabled: []domain.ModelRef{ref(t, "lit", "lit-image")}},
			provider: "lit", model: "lit-chat",
		},
		{
			name:     "disabling under another provider does not reach this one",
			lookup:   fakeLookup{disabled: []domain.ModelRef{ref(t, "dark", "lit-chat")}},
			provider: "lit", model: "lit-chat",
		},
		{
			name:     "a failed disabled read routes rather than locking every provider out",
			lookup:   fakeLookup{err: errors.New("catalog read failed")},
			provider: "lit", model: "lit-chat",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolver, err := NewResolver(catalogIndex(t), tc.lookup)
			if err != nil {
				t.Fatalf("NewResolver() error = %v", err)
			}
			resolution, resolveErr := resolver.ResolvePartsForKind(
				context.Background(), tc.provider, tc.model, KindChat,
			)
			if tc.wantErr {
				if resolveErr == nil {
					t.Fatalf("ResolvePartsForKind() served a disabled model: %+v", resolution)
				}
				if code := AsError(resolveErr).Code; code != CodeModelNotFound {
					t.Fatalf("AsError().Code = %q, want %q", code, CodeModelNotFound)
				}
				return
			}
			if resolveErr != nil {
				t.Fatalf("ResolvePartsForKind() error = %v", resolveErr)
			}
			if resolution.Provider.ID != tc.provider || resolution.ModelID != tc.model {
				t.Fatalf("ResolvePartsForKind() = %s/%s, want %s/%s",
					resolution.Provider.ID, resolution.ModelID, tc.provider, tc.model)
			}
		})
	}
}

func TestResolver_DisabledRefusesRoutingByEveryProviderSpelling(t *testing.T) {
	// The listing hides a model disabled under any spelling the entry answers to.
	// Routing has to answer the same way, or the model leaves the menu and stays
	// billed.
	cases := []struct {
		name        string
		disabledAs  string
		clientNames string
		model       string
	}{
		{"the canonical id the registry keys by", "lit", "lit", "lit-chat"},
		{"the alias, client naming the canonical id", "lt", "lit", "lit-chat"},
		{"the alias, client naming the alias", "lt", "lt", "lit-chat"},
		{"the node prefix, client naming the node id", nodePfx, nodeID, "work-model"},
		{"the node prefix, client naming the prefix", nodePfx, nodePfx, "work-model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := fakeLookup{disabled: []domain.ModelRef{ref(t, tc.disabledAs, tc.model)}}
			resolver, err := NewResolver(catalogIndex(t), lookup)
			if err != nil {
				t.Fatalf("NewResolver() error = %v", err)
			}
			_, err = resolver.ResolvePartsForKind(context.Background(), tc.clientNames, tc.model, KindChat)
			if err == nil {
				t.Fatal("ResolvePartsForKind() served the model, and the listing already hides it")
			}
			if code := AsError(err).Code; code != CodeModelNotFound {
				t.Fatalf("ResolvePartsForKind() code = %q, want %q", code, CodeModelNotFound)
			}
		})
	}
}

func TestResolver_DisabledRefusesComboMembers(t *testing.T) {
	lookup := fakeLookup{
		combos:   map[string]domain.Combo{"lit-combo": comboRow("lit-combo", "lit/lit-chat")},
		disabled: []domain.ModelRef{ref(t, "lit", "lit-chat")},
	}
	resolver, err := NewResolver(catalogIndex(t), lookup)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), "lit-combo"); AsError(err).Code != CodeModelNotFound {
		t.Fatalf("Resolve() on a combo whose member is disabled: code = %q, want %q",
			AsError(err).Code, CodeModelNotFound)
	}
}
