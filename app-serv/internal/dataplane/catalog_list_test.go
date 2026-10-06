// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/catalog_list_test.go
// @for       Table-driven tests for which models GET /api/v1/models is allowed to name.
// @uses      context, strings, testing, internal/domain, internal/registry
// @reason    SPEC-API-001 §7.15 publishes the list as what the router can answer, so two rules decide a row and neither was pinned before: a provider with no endpoint the router would pick is not a provider a client can use (draft 021 F8), and a model an operator added is listable even when the provider itself declares no catalog (draft 021 F10). Listing either wrong makes the picker and the router disagree, which is the failure an operator experiences as a model that will not send.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package dataplane

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

const (
	nodeID    = "openai-compatible-node1"
	nodePfx   = "th-1"
	deadOwner = "nowhere"
)

// catalogIndex covers the four shapes a listed row can come from: a keyed provider with
// endpoints, a keyed provider without any, an entry the router serves on a synthesized
// endpoint, and a custom node whose model list is the operator's own rows.
func catalogIndex(t *testing.T) *registry.Index {
	t.Helper()
	index, err := registry.NewIndex(registry.Document{Providers: []registry.Provider{
		{
			ID: "lit", Category: "apikey", Aliases: []string{"lt"},
			Transport: registry.Transport{Format: registry.DefaultFormat, BaseURL: "https://lit.test/v1"},
			Models:    []registry.Model{{ID: "lit-chat"}, {ID: "lit-image", Kind: "image"}},
		},
		{
			ID: "dark", Category: "apikey",
			Transport: registry.Transport{Format: registry.DefaultFormat, BaseURL: "https://dark.test/v1"},
			Models:    []registry.Model{{ID: "dark-chat"}},
		},
		{
			ID: "free", NoAuth: true, Category: "free",
			Transport: registry.Transport{Format: registry.DefaultFormat, BaseURL: "https://free.test/v1"},
			Models:    []registry.Model{{ID: "free-chat"}},
		},
		{
			ID: nodeID, Alias: nodePfx, Custom: true, Category: "apikey",
			Display:   registry.Display{Name: "TH HARBOR 1"},
			Transport: registry.Transport{Format: registry.DefaultFormat, BaseURL: "https://node.test/v1"},
		},
		{
			ID: "gemini-ish", Category: "apikey", PassthroughModels: true,
			Transport: registry.Transport{Format: "gemini", BaseURL: "https://g.test/v1"},
			Models:    []registry.Model{{ID: "no-translator"}},
		},
	}})
	if err != nil {
		t.Fatalf("building index: %v", err)
	}
	return index
}

// ref builds one stored provider/model pair, which is what both the disabled set and
// the operator's custom rows are made of.
func ref(t *testing.T, providerID, modelID string) domain.ModelRef {
	t.Helper()
	created, err := domain.NewModelRef(providerID, modelID)
	if err != nil {
		t.Fatalf("building ref %s/%s: %v", providerID, modelID, err)
	}
	return created
}

func TestModelList_ActiveProvidersAndCustomModels(t *testing.T) {
	live := map[string]bool{"lit": true, "dark": false, nodeID: true}

	// `free` needs no credential, so the router serves it on a synthesized endpoint and
	// it lists in every case below whatever the active map says. The case that isolates
	// that rule is the second one; here it is the constant row each expectation carries.
	cases := []struct {
		name     string
		custom   []domain.ModelRef
		active   map[string]bool
		disabled []domain.ModelRef
		combos   map[string]domain.Combo
		want     []string
	}{
		{
			name:   "a provider with no candidate endpoint lists nothing",
			active: live,
			want:   []string{"free/free-chat", "lit/lit-chat", nodePfx + "/kept"},
			custom: []domain.ModelRef{ref(t, nodeID, "kept")},
		},
		{
			name:   "an entry the router serves without a credential lists with no endpoint row",
			active: map[string]bool{},
			want:   []string{"free/free-chat"},
		},
		{
			name:   "a declared image model stays out of a chat list",
			active: map[string]bool{"lit": true},
			want:   []string{"free/free-chat", "lit/lit-chat"},
		},
		{
			name:   "a custom row stored under the node prefix surfaces in that prefix spelling",
			active: map[string]bool{nodeID: true},
			custom: []domain.ModelRef{ref(t, nodePfx, "deep-2")},
			want:   []string{"free/free-chat", nodePfx + "/deep-2"},
		},
		{
			name:   "a custom row under a provider with no candidate endpoint stays hidden",
			active: map[string]bool{"lit": true, "dark": false},
			custom: []domain.ModelRef{ref(t, "dark", "dark-custom")},
			want:   []string{"free/free-chat", "lit/lit-chat"},
		},
		{
			name:     "a disabled custom row is hidden from the list as it is from routing",
			active:   map[string]bool{"lit": true},
			custom:   []domain.ModelRef{ref(t, "lit", "gone")},
			disabled: []domain.ModelRef{ref(t, "lit", "gone")},
			want:     []string{"free/free-chat", "lit/lit-chat"},
		},
		{
			// §7.6 stores a disabled pair with the spelling the operator typed, so a
			// disable written as the node prefix must hide the row the list builds
			// from the node id.
			name:     "a custom row disabled under the node prefix is hidden",
			active:   map[string]bool{nodeID: true},
			custom:   []domain.ModelRef{ref(t, nodeID, "kept")},
			disabled: []domain.ModelRef{ref(t, nodePfx, "kept")},
			want:     []string{"free/free-chat"},
		},
		{
			name:     "a declared model disabled under a registry alias is hidden",
			active:   map[string]bool{"lit": true},
			disabled: []domain.ModelRef{ref(t, "lt", "lit-chat")},
			want:     []string{"free/free-chat"},
		},
		{
			name:   "a custom row naming a provider the registry does not know is not invented",
			active: map[string]bool{"lit": true},
			custom: []domain.ModelRef{ref(t, deadOwner, "x")},
			want:   []string{"free/free-chat", "lit/lit-chat"},
		},
		{
			name:   "a custom row duplicating a declared model yields one row",
			active: map[string]bool{"lit": true},
			custom: []domain.ModelRef{ref(t, "lit", "lit-chat")},
			want:   []string{"free/free-chat", "lit/lit-chat"},
		},
		{
			name:   "a combo lists even when every provider behind it is dark",
			active: map[string]bool{},
			combos: map[string]domain.Combo{"pi-agent": comboRow("pi-agent", "lit/lit-chat")},
			want:   []string{"free/free-chat", "pi-agent"},
		},
		{
			name:   "a provider without a translator lists nothing even when active",
			active: map[string]bool{"gemini-ish": true, "lit": true},
			want:   []string{"free/free-chat", "lit/lit-chat"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := fakeLookup{
				combos:   tc.combos,
				aliases:  map[string]string{},
				custom:   tc.custom,
				active:   tc.active,
				disabled: tc.disabled,
			}
			resolver, err := NewResolver(catalogIndex(t), lookup)
			if err != nil {
				t.Fatalf("building resolver: %v", err)
			}

			list, err := resolver.ModelList(context.Background())
			if err != nil {
				t.Fatalf("ModelList: %v", err)
			}
			got := make([]string, 0, len(list.Data))
			for _, entry := range list.Data {
				got = append(got, entry.ID)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("listed = %v, want %v", got, tc.want)
			}
			for i := 1; i < len(list.Data); i++ {
				if list.Data[i-1].ID > list.Data[i].ID {
					t.Fatalf("list not sorted: %q before %q", list.Data[i-1].ID, list.Data[i].ID)
				}
			}
		})
	}
}
