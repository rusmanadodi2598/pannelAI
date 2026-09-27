// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_encoding_test.go
// @for       The body obfuscation, against vectors the reference produced.
//
// @uses      encoding/json, os, path/filepath, strings, testing.
// @reason    The obfuscation is a vendor's algorithm with no room for taste: one
//
//	reordered third or one substituted character off and the WAF reads a
//	plaintext body. The expected values are the reference encoder's own
//	output, captured in `testdata/qoder_encoding_vectors.json` and read
//	back here, so a drift is caught rather than self-confirmed — and
//	not re-typed by hand, which is how a vector first went wrong.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// encodingVectors is the captured file: where the values came from, and the pairs.
type encodingVectors struct {
	Source   string `json:"source"`
	Captured string `json:"captured"`
	Vectors  []struct {
		Plain   string `json:"plain"`
		Encoded string `json:"encoded"`
	} `json:"vectors"`
}

func loadEncodingVectors(t *testing.T) encodingVectors {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "qoder_encoding_vectors.json"))
	if err != nil {
		t.Fatalf("reading the encoder vectors: %v", err)
	}
	var doc encodingVectors
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decoding the encoder vectors: %v", err)
	}
	if doc.Source == "" || len(doc.Vectors) == 0 {
		t.Fatalf("the vector file names no source or carries no cases: %+v", doc)
	}
	return doc
}

// TestQoderEncodeBodyMatchesTheReference runs every captured pair. A case name is
// the first characters of its plaintext so a failure points at the body, not at a
// label that was written for another one.
func TestQoderEncodeBodyMatchesTheReference(t *testing.T) {
	doc := loadEncodingVectors(t)
	t.Logf("%d vectors captured %s from %s", len(doc.Vectors), doc.Captured, doc.Source)

	for _, testCase := range doc.Vectors {
		name := testCase.Plain
		switch {
		case name == "":
			name = "an empty body"
		case len(name) > 24:
			name = name[:24] + "…"
		}
		t.Run(name, func(t *testing.T) {
			if got := qoderEncodeBody([]byte(testCase.Plain)); got != testCase.Encoded {
				t.Fatalf("encode(%q) = %q, want %q", testCase.Plain, got, testCase.Encoded)
			}
		})
	}
}

// TestQoderEncodeBodyKeepsLength pins the property the reference relies on: the
// obfuscation is a substitution, so a body of n bytes arrives as n characters — the
// base64 length of those bytes — which is what Cosy-Bodylength and the outbound
// Content-Length then describe.
func TestQoderEncodeBodyKeepsLength(t *testing.T) {
	for _, size := range []int{0, 1, 2, 3, 5, 12, 97, 513} {
		encoded := qoderEncodeBody([]byte(strings.Repeat("x", size)))
		if want := (size + 2) / 3 * 4; len(encoded) != want {
			t.Fatalf("size %d encoded to %d characters, want %d", size, len(encoded), want)
		}
	}
}

// TestQoderEncodeBodyCarriesNoPadding asserts the other rule the two alphabets
// encode: "=" maps to "$", so an encoded body never carries a character that would
// need escaping on the way out.
func TestQoderEncodeBodyCarriesNoPadding(t *testing.T) {
	for _, plain := range []string{"a", "ab", "abc", "abcd", strings.Repeat("x", 97)} {
		if got := qoderEncodeBody([]byte(plain)); strings.ContainsRune(got, '=') {
			t.Fatalf("encode(%q) = %q, want no padding character", plain, got)
		}
	}
}
