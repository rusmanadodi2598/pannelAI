// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/index_test.go
// @for       Table-driven tests for identifier resolution and the collision
//
//	rules the index enforces.
//
// @uses      testing, internal/registry.
// @reason    Every model string the data plane receives is resolved through this
//
//	index, so an ambiguous identifier is a routing fault that only shows
//	up in production traffic; these tests pin the resolution order and
//	the refusals.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import "testing"

func TestNewIndex_ResolvesEveryIdentifierShape(t *testing.T) {
	idx, err := loadFixture(t, `
revision: test@1
providers:
  - id: deepseek
    alias: ds
    aliases:
      - deep
    category: apikey
    models:
      - id: deepseek-chat
        name: DeepSeek Chat
  - id: custom-node
    alias: mycorp
    category: apikey
    transport:
      format: openai
      base_url: https://mycorp.test/v1
`)
	if err != nil {
		t.Fatalf("load error = %v", err)
	}

	cases := []struct {
		name   string
		lookup string
		wantID string
		wantOK bool
	}{
		{name: "by id", lookup: "deepseek", wantID: "deepseek", wantOK: true},
		{name: "by alias", lookup: "ds", wantID: "deepseek", wantOK: true},
		{name: "by extra alias", lookup: "deep", wantID: "deepseek", wantOK: true},
		{name: "by the second provider's id", lookup: "custom-node", wantID: "custom-node", wantOK: true},
		{name: "by the second provider's alias", lookup: "mycorp", wantID: "custom-node", wantOK: true},
		{name: "unknown id", lookup: "nope"},
		{name: "resolution is case sensitive", lookup: "DeepSeek"},
		{name: "an empty name resolves to nothing", lookup: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := idx.Provider(tc.lookup)
			if ok != tc.wantOK {
				t.Fatalf("Provider(%q) ok = %v, want %v", tc.lookup, ok, tc.wantOK)
			}
			if ok && got.ID != tc.wantID {
				t.Fatalf("Provider(%q).ID = %q, want %q", tc.lookup, got.ID, tc.wantID)
			}
		})
	}

	if got := idx.Revision(); got != "test@1" {
		t.Fatalf("Revision() = %q, want test@1", got)
	}
	if got := idx.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}
	if got := len(idx.All()); got != 2 {
		t.Fatalf("All() returned %d providers, want 2", got)
	}
	if got := idx.Categories(); len(got) != 1 || got[0] != "apikey" {
		t.Fatalf("Categories() = %v, want [apikey]", got)
	}
	if got := len(idx.ByCategory("apikey")); got != 2 {
		t.Fatalf("ByCategory(apikey) = %d providers, want 2", got)
	}
	if got := len(idx.ByCategory("oauth")); got != 0 {
		t.Fatalf("ByCategory(oauth) = %d providers, want 0", got)
	}
}

func TestNewIndex_RejectsAmbiguousAndMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "two providers share an id",
			yaml: `
providers:
  - id: dup
    category: apikey
  - id: dup
    category: apikey
`,
		},
		{
			name: "two providers share an alias",
			yaml: `
providers:
  - id: a
    alias: shared
    category: apikey
  - id: b
    aliases:
      - shared
    category: apikey
`,
		},
		{
			name: "an extra alias collides with another provider's alias",
			yaml: `
providers:
  - id: a
    alias: one
    category: apikey
  - id: b
    alias: two
    aliases:
      - one
    category: apikey
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := loadFixture(t, tc.yaml); err == nil {
				t.Fatal("load = nil error, want a collision error")
			}
		})
	}
}
