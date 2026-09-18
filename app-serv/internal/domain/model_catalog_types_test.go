// Package domain holds entities, value objects, and the ubiquitous language
//
// @file      internal/domain/model_catalog_types_test.go
// @for       The custom model, alias, and merged catalog row tests.
// @uses      testing, time.
// @reason    The reference tests and the row tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the row cases moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"testing"
)

// TestNewModelAlias pins the alias shape: it may not be an alias for an alias,
// and its target may be a model, a combo name, or an alias.
func TestNewModelAlias(t *testing.T) {
	cases := []struct {
		name    string
		alias   string
		target  string
		wantErr string
	}{
		{name: "a target model", alias: "fast", target: "openai/gpt-4o-mini"},
		{name: "a target combo", alias: "daily", target: "fallback-combo"},
		{name: "an empty alias", alias: " ", target: "openai/gpt-4o", wantErr: "alias is required"},
		{name: "an alias with a slash", alias: "a/b", target: "openai/gpt-4o", wantErr: "must not contain a slash"},
		{name: "an empty target", alias: "fast", target: "", wantErr: "model reference is required"},
		{name: "a target with whitespace", alias: "fast", target: "openai/gpt 4o", wantErr: "must not contain whitespace"},
		{name: "a target with a bad character", alias: "fast", target: "bad!", wantErr: "must be provider/model"},
		{
			name: "an over-long alias", alias: strings.Repeat("a", 121), target: "openai/gpt-4o",
			wantErr: "at most 120",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			alias, err := NewModelAlias(tc.alias, tc.target)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("NewModelAlias(%q, %q) accepted", tc.alias, tc.target)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("NewModelAlias(%q, %q) error = %v, want %q", tc.alias, tc.target, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewModelAlias(%q, %q) error = %v", tc.alias, tc.target, err)
			}
			if alias.Alias() != tc.alias || alias.Target() != tc.target {
				t.Fatalf("NewModelAlias() = %q → %q, want %q → %q", alias.Alias(), alias.Target(), tc.alias, tc.target)
			}
		})
	}
}

// TestNewCatalogModel pins the merged row's construction.
func TestNewCatalogModel(t *testing.T) {
	ref, err := NewModelRef("openai", "gpt-4o")
	if err != nil {
		t.Fatalf("NewModelRef() error = %v", err)
	}
	model := NewCatalogModel(ref, "GPT-4o", "llm", NewModelCapabilities("vision", "tools"), CatalogSourceRegistry)
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"provider", model.ProviderID(), "openai"},
		{"model", model.ModelID(), "gpt-4o"},
		{"display name", model.DisplayName(), "GPT-4o"},
		{"kind", model.Kind(), "llm"},
		{"source", string(model.Source()), "registry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
	if !model.Capabilities().Has("tools") {
		t.Fatalf("Capabilities() = %v, want tools", model.Capabilities().List())
	}
}
