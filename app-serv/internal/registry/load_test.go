// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/load_test.go
// @for       Table-driven tests for document decoding, indexing, and synthesis.
// @uses      testing, internal/registry.
// @reason    SPEC-API-001 §6 makes the registry the source of truth for every
//
//	provider, so a decode that silently drops a field is a routing bug
//	that surfaces as a wrong upstream call; these tests pin the decode
//	contract, the index, and the custom-node synthesis.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import (
	"testing"
)

// The fixtures below are raw YAML rather than marshalled Go structs on purpose:
// marshalling would round-trip whatever the structs already say, which is the
// assumption under test. Each fixture mirrors a real shape from the reference
// registry, including its quirks (a scalar `scopes`, a `retry` in three forms,
// a date-shaped header value).

// loadFixture decodes a document and builds its index, which is the pair a boot
// performs. Assertions run against the index, because that is what the rest of
// the system reads: a value the decoder accepted but the index dropped is
// exactly the drift these tests exist to catch.
func loadFixture(t *testing.T, raw string) (*Index, error) {
	t.Helper()
	doc, err := decode([]byte(raw))
	if err != nil {
		return nil, err
	}
	return NewIndex(doc)
}

func TestLoad_DecodesDocumentShape(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		check   func(t *testing.T, idx *Index)
		wantErr bool
	}{
		{
			name: "minimal provider with only the required fields",
			yaml: `
revision: test@1
providers:
  - id: bare
    category: apikey
    transport:
      format: openai
      base_url: https://example.test/v1/chat/completions
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				if idx.Revision() != "test@1" {
					t.Fatalf("revision = %q, want test@1", idx.Revision())
				}
				p, ok := idx.Provider("bare")
				if !ok || p.ID != "bare" {
					t.Fatalf("Provider(bare) = (%+v, %v), want the bare provider", p, ok)
				}
			},
		},
		{
			name: "transport defaults applied when format is absent",
			yaml: `
providers:
  - id: p
    category: apikey
    transport:
      base_url: https://example.test/v1/chat/completions
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				if got := p.Transport.Format; got != "openai" {
					t.Fatalf("format = %q, want the default openai", got)
				}
			},
		},
		{
			name: "auth type is derived from the declared shape",
			yaml: `
providers:
  - id: keyed
    category: apikey
  - id: flowing
    category: oauth
    oauth:
      token_url: https://example.test/token
  - id: anonymous
    category: free
    no_auth: true
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				for id, want := range map[string]string{
					"keyed": AuthAPIKey, "flowing": AuthOAuth, "anonymous": AuthNone,
				} {
					p, ok := idx.Provider(id)
					if !ok {
						t.Fatalf("provider %q is missing", id)
					}
					if p.AuthType != want {
						t.Fatalf("%s auth_type = %q, want %q", id, p.AuthType, want)
					}
				}
			},
		},
		{
			name: "an explicitly declared auth type is not overridden",
			yaml: `
providers:
  - id: both
    category: oauth
    auth_type: api_key
    oauth:
      token_url: https://example.test/token
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("both")
				if p.AuthType != AuthAPIKey {
					t.Fatalf("auth_type = %q, want the declared api_key", p.AuthType)
				}
			},
		},
		{
			name: "no_auth declared on the transport is honoured",
			yaml: `
providers:
  - id: p
    category: free
    transport:
      no_auth: true
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				if p.AuthType != AuthNone {
					t.Fatalf("auth_type = %q, want no_auth", p.AuthType)
				}
			},
		},
		{
			name: "unknown key is rejected rather than silently dropped",
			yaml: `
providers:
  - id: p
    category: apikey
    this_key_does_not_exist: 1
`,
			wantErr: true,
		},
		{
			name:    "empty document is rejected",
			yaml:    "",
			wantErr: true,
		},
		{
			name: "provider without an id is rejected",
			yaml: `
providers:
  - category: apikey
`,
			wantErr: true,
		},
		{
			name: "provider without a category is rejected",
			yaml: `
providers:
  - id: p
`,
			wantErr: true,
		},
		{
			name: "document with no providers is rejected",
			yaml: `
revision: test@1
`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx, err := loadFixture(t, tc.yaml)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("load = %+v with nil error, want an error", idx)
				}
				return
			}
			if err != nil {
				t.Fatalf("load error = %v", err)
			}
			tc.check(t, idx)
		})
	}
}
