// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/decode_retry_test.go
// @for       Decode tests for the retry field, whose shape varies across the
// reference registry.
// @uses      testing, internal/registry.
// @reason    The reference writes retry as a bare count, a per-status count, and
// a per-status attempts object, and a decoder that accepts only one form
// silently drops the others, so every variant is pinned here rather than
// trusted.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import "testing"

func TestDecode_RetryShapes(t *testing.T) {
	cases := []struct {
		name  string
		yaml  string
		check func(t *testing.T, idx *Index)
	}{
		{
			name: "retry accepts a bare count",
			yaml: `
providers:
  - id: p
    category: apikey
    transport:
      base_url: https://example.test
      retry: 4
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				if got := p.Transport.Retry.Attempts(500); got != 4 {
					t.Fatalf("attempts = %d, want 4", got)
				}
			},
		},
		{
			name: "retry accepts a per-status count",
			yaml: `
providers:
  - id: p
    category: apikey
    transport:
      base_url: https://example.test
      retry:
        "429": 3
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				if got := p.Transport.Retry.Attempts(429); got != 3 {
					t.Fatalf("attempts(429) = %d, want 3", got)
				}
				if got := p.Transport.Retry.Attempts(500); got != 0 {
					t.Fatalf("attempts(500) = %d, want 0 when only 429 is declared", got)
				}
			},
		},
		{
			name: "retry accepts a per-status attempts object",
			yaml: `
providers:
  - id: p
    category: apikey
    transport:
      base_url: https://example.test
      retry:
        "429":
          attempts: 3
        "503":
          attempts: 2
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				for status, want := range map[int]int{429: 3, 503: 2} {
					if got := p.Transport.Retry.Attempts(status); got != want {
						t.Fatalf("attempts(%d) = %d, want %d", status, got, want)
					}
				}
			},
		},
		{
			name: "retry mixing both shapes",
			yaml: `
providers:
  - id: p
    category: apikey
    transport:
      base_url: https://example.test
      retry:
        "429": 3
        "503":
          attempts: 1
`,
			check: func(t *testing.T, idx *Index) {
				t.Helper()
				p, _ := idx.Provider("p")
				for status, want := range map[int]int{429: 3, 503: 1} {
					if got := p.Transport.Retry.Attempts(status); got != want {
						t.Fatalf("attempts(%d) = %d, want %d", status, got, want)
					}
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
