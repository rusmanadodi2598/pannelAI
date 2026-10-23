// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/decode_test.go
// @for       Strict decode and validation of an mpv2 document.
// @uses      testing, strings, errors.
// @reason    A builder reply that slips past validation reaches a paid call, so every rejection the spec names is pinned here.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/masterprompt"
)

func TestDecode_GivenDocumentVariations_ThenTheDocumentIsAcceptedOrRefused(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(map[string]any)
		wantCode string
	}{
		{
			name:   "valid image document is accepted",
			mutate: func(map[string]any) {},
		},
		{
			name: "unknown field is refused",
			mutate: func(doc map[string]any) {
				doc["mood_board"] = "invented section"
			},
			wantCode: masterprompt.CodeInvalid,
		},
		{
			name: "missing task is refused",
			mutate: func(doc map[string]any) {
				delete(doc, "task")
			},
			wantCode: masterprompt.CodeInvalid,
		},
		{
			name: "aspect mismatch between stop conditions and output is refused",
			mutate: func(doc map[string]any) {
				doc["stop_conditions"].(map[string]any)["aspect_ratio"] = "9:16"
			},
			wantCode: masterprompt.CodeInvalid,
		},
		{
			name: "a url in a field is refused",
			mutate: func(doc map[string]any) {
				doc["context"].(map[string]any)["environment"] = "see https://example.com/street for the look"
			},
			wantCode: masterprompt.CodeUnsupported,
		},
		{
			name: "reasoning without a context element is refused",
			mutate: func(doc map[string]any) {
				doc["reasoning"] = "this choice simply works well for everyone involved and reads clearly at any size"
			},
			wantCode: masterprompt.CodeInvalid,
		},
		{
			name: "image document with a duration is refused",
			mutate: func(doc map[string]any) {
				doc["output"].(map[string]any)["duration_s"] = 6
			},
			wantCode: masterprompt.CodeUnsupported,
		},
		{
			name: "count above one is refused",
			mutate: func(doc map[string]any) {
				doc["output"].(map[string]any)["count"] = 2
			},
			wantCode: masterprompt.CodeUnsupported,
		},
		{
			name: "role naming a single product is remapped to the family descriptor",
			mutate: func(doc map[string]any) {
				doc["role"] = "iphone photographer with product focus"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := imageDocument(t)
			tc.mutate(doc)
			raw := encode(t, doc)

			decoded, err := masterprompt.Decode(raw)

			if tc.wantCode != "" {
				var promptErr *masterprompt.Error
				if !errors.As(err, &promptErr) {
					t.Fatalf("Decode() error = %v, want a prompt error", err)
				}
				if promptErr.Code != tc.wantCode {
					t.Fatalf("code = %q, want %q", promptErr.Code, tc.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("Decode() error = %v, want nil", err)
			}
			if decoded.Version != masterprompt.Version {
				t.Fatalf("version = %q, want %q", decoded.Version, masterprompt.Version)
			}
			if strings.Contains(decoded.Role, "iphone") {
				t.Fatalf("role = %q, want the family descriptor instead of a product name", decoded.Role)
			}
		})
	}
}

func TestValidate_GivenNormalizedText_ThenWhitespaceAndControlCharactersAreGone(t *testing.T) {
	doc := imageDocument(t)
	doc["task"] = "  create	 a hero image\n of an orange cat  "

	decoded, err := masterprompt.Decode(encode(t, doc))
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if decoded.Task != "create a hero image of an orange cat" {
		t.Fatalf("task = %q, want normalized single spaces", decoded.Task)
	}
}
