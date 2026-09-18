// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/decode_test.go
// @for       Decode tests for the fields whose shape varies across the reference
//
//	registry: scopes, headers, and the model catalog fields.
//
// @uses      testing, internal/registry.
// @reason    These are the fields the reference writes in more than one YAML
//
//	shape, and each shape was a real porting defect. A decoder that
//	accepts only one form silently drops the others, so every variant
//	is pinned here rather than trusted.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import "testing"

func TestDecode_VariableShapeFields(t *testing.T) {
	cases := []struct {
		name  string
		yaml  string
		check func(t *testing.T, idx *Index)
	}{
		{
			name: "scopes accepts a scalar and a list alike",
			yaml: `
providers:
  - id: scalar
    category: oauth
    oauth:
      scopes: read:user
  - id: list
    category: oauth
    oauth:
      scopes:
        - a
        - b
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				scalar, _ := idx.Provider("scalar")
				if got := scalar.OAuth.ScopeList(); len(got) != 1 || got[0] != "read:user" {
					t.Fatalf("scalar scopes = %v, want [read:user]", got)
				}
				list, _ := idx.Provider("list")
				if got := list.OAuth.ScopeList(); len(got) != 2 {
					t.Fatalf("list scopes = %v, want two entries", got)
				}
			},
		},
		{
			name: "a singular scope is honoured when scopes is absent",
			yaml: `
providers:
  - id: p
    category: oauth
    oauth:
      scope: openid profile email
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				got := p.OAuth.ScopeList()
				if len(got) != 1 || got[0] != "openid profile email" {
					t.Fatalf("ScopeList() = %v, want the singular scope verbatim", got)
				}
			},
		},
		{
			name: "an empty scope list resolves to nothing",
			yaml: `
providers:
  - id: p
    category: oauth
    oauth:
      scopes: ""
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				if got := p.OAuth.ScopeList(); len(got) != 0 {
					t.Fatalf("ScopeList() = %v, want nothing for an empty scope", got)
				}
			},
		},
		{
			name: "header values keep their string type",
			yaml: `
providers:
  - id: p
    category: apikey
    transport:
      base_url: https://example.test
      headers:
        Anthropic-Version: "2023-06-01"
        X-Retry-Count: "0"
        X-Literal-True: "true"
        X-Comma-Separated: a,b,c
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				for key, want := range map[string]string{
					"Anthropic-Version": "2023-06-01",
					"X-Retry-Count":     "0",
					"X-Literal-True":    "true",
					"X-Comma-Separated": "a,b,c",
				} {
					if got := p.Transport.Headers[key]; got != want {
						t.Fatalf("header %s = %q, want %q", key, got, want)
					}
				}
			},
		},
		{
			name: "model upstream id falls back to the exposed id",
			yaml: `
providers:
  - id: p
    category: apikey
    models:
      - id: exposed
        name: Exposed
      - id: alias
        name: Alias
        upstream_model_id: real-model
      - id: image
        name: Image
        kind: image
      - id: media
        name: Media
        kind: llm
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				if got := p.Models[0].UpstreamID(); got != "exposed" {
					t.Fatalf("UpstreamID() = %q, want the exposed id", got)
				}
				if got := p.Models[1].UpstreamID(); got != "real-model" {
					t.Fatalf("UpstreamID() = %q, want real-model", got)
				}
				if !p.Models[0].IsChat() {
					t.Fatal("a model with no kind must be routable as chat")
				}
				if !p.Models[3].IsChat() {
					t.Fatal("an explicit llm kind must stay routable as chat")
				}
				if p.Models[2].IsChat() {
					t.Fatal("a model with kind image must not be routable as chat")
				}
			},
		},
		{
			name: "an explicitly empty params list is preserved",
			yaml: `
providers:
  - id: p
    category: apikey
    models:
      - id: strict
        name: Strict
        params: []
      - id: loose
        name: Loose
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				if p.Models[0].Params == nil {
					t.Fatal("an empty params list must stay non-nil, so declared-empty is distinct from absent")
				}
				if p.Models[1].Params != nil {
					t.Fatal("an absent params list must stay nil")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx, err := loadFixture(t, tc.yaml)
			if err != nil {
				t.Fatalf("load error = %v", err)
			}
			tc.check(t, idx)
		})
	}
}
