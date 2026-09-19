// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/resolve_fixtures_test.go
// @for       The lookup double and registry index the resolution tests build on.
// @uses      context, testing, internal/domain, internal/registry
// @reason    SPEC-API-001 §7.15 fixes the resolution order (combo, alias, provider/model) and the
//
//	models list, and both read a lookup the resolver does not own. The double and the
//	registry index live here so the resolution tests read as the rule they pin.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// fakeLookup answers the catalog questions from fixed sets.
type fakeLookup struct {
	combos   map[string]domain.Combo
	aliases  map[string]string
	disabled []domain.ModelRef
	err      error
}

func (l fakeLookup) Combo(_ context.Context, name string) (domain.Combo, bool, error) {
	if l.err != nil {
		return domain.Combo{}, false, l.err
	}
	combo, ok := l.combos[name]
	return combo, ok, nil
}

func (l fakeLookup) Alias(_ context.Context, name string) (string, bool, error) {
	if l.err != nil {
		return "", false, l.err
	}
	target, ok := l.aliases[name]
	return target, ok, nil
}

func (l fakeLookup) Disabled(_ context.Context, providerID, modelID string) (bool, error) {
	if l.err != nil {
		return false, l.err
	}
	for _, ref := range l.disabled {
		if ref.ProviderID() == providerID && ref.ModelID() == modelID {
			return true, nil
		}
	}
	return false, nil
}

func (l fakeLookup) DisabledPairs(context.Context) ([]domain.ModelRef, error) {
	return l.disabled, l.err
}

func (l fakeLookup) ComboNames(context.Context) ([]string, error) {
	if l.err != nil {
		return nil, l.err
	}
	names := make([]string, 0, len(l.combos))
	for name := range l.combos {
		names = append(names, name)
	}
	return names, nil
}

// testIndex builds a registry index covering the routable and non-routable cases
// from one YAML document, so the resolution tests run against the loader the router
// uses rather than a hand-built struct.
func testIndex(t *testing.T) *registry.Index {
	t.Helper()
	index, err := registry.NewIndex(registry.Document{Providers: []registry.Provider{
		{
			ID: "provider-a", Category: "apikey", Alias: "pa", PassthroughModels: true,
			Transport: registry.Transport{Format: registry.DefaultFormat, BaseURL: "https://a.test/v1"},
		},
		{
			ID: "claude-only", Category: "apikey", PassthroughModels: true,
			Transport: registry.Transport{Format: "claude", BaseURL: "https://c.test/v1/messages"},
		},
		{
			ID: "gated", Category: "apikey", Hidden: true,
			Transport: registry.Transport{Format: "claude", BaseURL: "https://g.test/v1"},
			Models:    []registry.Model{{ID: "secret-model"}},
		},
		{
			ID: "connector-only", Category: "oauth", AuthType: registry.AuthOAuth,
			Transport: registry.Transport{Format: "kiro", BaseURL: "https://k.test"},
		},
		{
			ID: "declared", Category: "apikey",
			Transport: registry.Transport{Format: registry.DefaultFormat, BaseURL: "https://d.test/v1"},
			Models: []registry.Model{
				{ID: "known-model"},
				{ID: "exposed-model", UpstreamModelID: "real-model"},
				{ID: "claude-native", TargetFormat: "claude"},
				{ID: "an-image", Kind: "image"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("building index: %v", err)
	}
	return index
}
