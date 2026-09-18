// Package domain holds entities, value objects, and the ubiquitous language
//
// @file      internal/domain/model_catalog_test.go
// @for       Table-driven tests for the model reference, capability set, custom (first half; split at the AGENTS.md §1.1 line limit).
// @uses      testing, time.
// @reason    Every catalog row, alias target, and combo reference is addressed
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"testing"
	"time"
)

// TestModelRef_Construction pins the pair rules, including the asymmetry that a
// model id may contain a slash while a provider id may not.
func TestModelRef_Construction(t *testing.T) {
	cases := []struct {
		name       string
		providerID string
		modelID    string
		wantErr    string
	}{
		{name: "a plain pair", providerID: "openai", modelID: "gpt-4o"},
		{name: "a model id with a slash", providerID: "openrouter", modelID: "openai/gpt-4o"},
		{name: "surrounding whitespace is trimmed", providerID: " openai ", modelID: " gpt-4o "},
		{name: "an empty provider", providerID: "", modelID: "gpt-4o", wantErr: "provider_id is required"},
		{name: "a whitespace provider", providerID: "   ", modelID: "gpt-4o", wantErr: "provider_id is required"},
		{name: "an empty model", providerID: "openai", modelID: "", wantErr: "model_id is required"},
		{name: "a provider with a slash", providerID: "open/ai", modelID: "gpt-4o", wantErr: "must not contain a slash"},
		{name: "a provider with whitespace", providerID: "open ai", modelID: "gpt-4o", wantErr: "must not contain a slash or whitespace"},
		{name: "a model with whitespace", providerID: "openai", modelID: "gpt 4o", wantErr: "must not contain whitespace"},
		{
			name: "an over-long model", providerID: "openai", modelID: strings.Repeat("m", 201),
			wantErr: "too long",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := NewModelRef(tc.providerID, tc.modelID)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("NewModelRef(%q, %q) accepted an invalid pair", tc.providerID, tc.modelID)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("NewModelRef(%q, %q) error = %v, want %q", tc.providerID, tc.modelID, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewModelRef(%q, %q) error = %v", tc.providerID, tc.modelID, err)
			}
			if ref.String() != strings.TrimSpace(tc.providerID)+"/"+strings.TrimSpace(tc.modelID) {
				t.Fatalf("String() = %q, want the trimmed pair", ref.String())
			}
			if ref.IsZero() {
				t.Fatal("IsZero() = true for a constructed reference")
			}
		})
	}
	if !(ModelRef{}).IsZero() {
		t.Fatal("IsZero() = false for the zero reference")
	}
	if got := (ModelRef{}).String(); got != "" {
		t.Fatalf("String() = %q for the zero reference, want empty", got)
	}
}

// TestParseModelRef pins the first-slash split, because a model id that carries
// its own slash is a real registry value here.
func TestParseModelRef(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantProv  string
		wantModel string
		wantErr   bool
	}{
		{name: "a simple pair", raw: "openai/gpt-4o", wantProv: "openai", wantModel: "gpt-4o"},
		{name: "splits on the first slash", raw: "openrouter/openai/gpt-4o", wantProv: "openrouter", wantModel: "openai/gpt-4o"},
		{name: "trims around the pair", raw: " openai/gpt-4o ", wantProv: "openai", wantModel: "gpt-4o"},
		{name: "a bare name is not a reference", raw: "daily", wantErr: true},
		{name: "an empty string is not a reference", raw: "", wantErr: true},
		{name: "a missing provider is not a reference", raw: "/gpt-4o", wantErr: true},
		{name: "a missing model is not a reference", raw: "openai/", wantErr: true},
		{name: "a nested empty model is not a reference", raw: "openai/gpt 4o", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := ParseModelRef(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseModelRef(%q) accepted %q", tc.raw, ref.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseModelRef(%q) error = %v", tc.raw, err)
			}
			if ref.ProviderID() != tc.wantProv || ref.ModelID() != tc.wantModel {
				t.Fatalf("ParseModelRef(%q) = %q/%q, want %q/%q", tc.raw, ref.ProviderID(), ref.ModelID(), tc.wantProv, tc.wantModel)
			}
		})
	}
}

// TestModelCapabilities_Canonical pins the normalization: a jsonb column
// round-trips to the same bytes and a membership test cannot disagree with a
// stored value over spelling.
func TestModelCapabilities_Canonical(t *testing.T) {
	cases := []struct {
		name    string
		input   []string
		want    []string
		has     string
		wantHas bool
	}{
		{name: "sorted and trimmed", input: []string{" video ", "audio"}, want: []string{"audio", "video"}, has: "audio", wantHas: true},
		{name: "case-insensitive duplicates collapse", input: []string{"vision", "VISION", "Vision"}, want: []string{"vision"}, has: "VISION", wantHas: true},
		{name: "empties are dropped", input: []string{"", "  ", "tools"}, want: []string{"tools"}, has: "tools", wantHas: true},
		{name: "no members", input: nil, want: []string{}, has: "vision", wantHas: false},
		{name: "a missing member", input: []string{"video"}, want: []string{"video"}, has: "vision", wantHas: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := NewModelCapabilities(tc.input...)
			got := set.List()
			if len(got) != len(tc.want) {
				t.Fatalf("List() = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("List() = %v, want %v", got, tc.want)
				}
			}
			if set.Has(tc.has) != tc.wantHas {
				t.Fatalf("Has(%q) = %t, want %t", tc.has, set.Has(tc.has), tc.wantHas)
			}
		})
	}
}

// TestModelCapabilities_ListIsACopy keeps a caller from reordering the value.
func TestModelCapabilities_ListIsACopy(t *testing.T) {
	set := NewModelCapabilities("a", "b")
	list := set.List()
	list[0] = "mutated"
	if set.List()[0] != "a" {
		t.Fatalf("List() exposed the value's own slice: %v", set.List())
	}
}

// TestNewCustomModel pins the custom row's validation.
func TestNewCustomModel(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		id          string
		providerID  string
		modelID     string
		displayName string
		wantErr     string
	}{
		{name: "a valid row", id: "mdl_1", providerID: "openai", modelID: "gpt-4o-mini", displayName: "GPT-4o mini"},
		{name: "a missing id", id: "", providerID: "openai", modelID: "x", displayName: "X", wantErr: "id is required"},
		{name: "a missing provider", id: "mdl_1", providerID: "", modelID: "x", displayName: "X", wantErr: "provider_id is required"},
		{name: "a missing model", id: "mdl_1", providerID: "openai", modelID: "", displayName: "X", wantErr: "model_id is required"},
		{name: "a missing display name", id: "mdl_1", providerID: "openai", modelID: "x", displayName: " ", wantErr: "display_name is required"},
		{name: "a control character in the name", id: "mdl_1", providerID: "openai", modelID: "x", displayName: "X\x00Y", wantErr: "control characters"},
		{
			name: "an over-long display name", id: "mdl_1", providerID: "openai", modelID: "x",
			displayName: strings.Repeat("n", 121), wantErr: "at most 120",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := NewCustomModel(tc.id, tc.providerID, tc.modelID, tc.displayName,
				NewModelCapabilities("vision"), now)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("NewCustomModel() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("NewCustomModel() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewCustomModel() error = %v", err)
			}
			if !model.Capabilities().Has("VISION") {
				t.Fatalf("Capabilities() = %v, want vision", model.Capabilities().List())
			}
			if !model.CreatedAt().Equal(now) {
				t.Fatalf("CreatedAt() = %v, want %v", model.CreatedAt(), now)
			}
		})
	}
}
