// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/fallback_test.go
// @for       The deterministic builder used when the model is unavailable.
// @uses      testing, strings, errors.
// @reason    A builder outage must still produce a usable draft, so the extraction rules are pinned and the honest label is checked.
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

func TestFallback_GivenUserText_ThenTheDocumentKeepsWhatItCanExtract(t *testing.T) {
	defaults := masterprompt.Defaults{AspectRatio: "16:9", Style: "cinematic photo"}

	cases := []struct {
		name        string
		text        string
		modality    string
		aspect      string
		medium      string
		duration    int
		wantDetails string
		wantCode    string
	}{
		{
			name:     "aspect token is read from the text",
			text:     "poster kucing oranye naik skateboard di Jakarta, aspect 9:16",
			modality: "image",
			aspect:   "9:16",
			medium:   "cinematic photo",
		},
		{
			name:     "style word is mapped to the closed vocabulary",
			text:     "ilustrasi watercolor teapot di meja kayu",
			modality: "image",
			aspect:   "16:9",
			medium:   "watercolor",
		},
		{
			name:     "duration word sets the video length",
			text:     "video reveal teapot keramik, 6 detik, aspect 9:16",
			modality: "video",
			aspect:   "9:16",
			medium:   "cinematic photo",
			duration: 6,
		},
		{
			name:        "camera move is kept as a detail",
			text:        "video produk sepatu, pan right, 5 detik",
			modality:    "video",
			aspect:      "16:9",
			medium:      "cinematic photo",
			duration:    5,
			wantDetails: "pan_right",
		},
		{
			name:     "no match falls back to the configured defaults",
			text:     "gambar abstrak untuk sampul album",
			modality: "image",
			aspect:   "16:9",
			medium:   "cinematic photo",
		},
		{
			name:     "empty text is refused",
			text:     "   ",
			modality: "image",
			wantCode: masterprompt.CodeEmpty,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := masterprompt.Fallback(tc.text, tc.modality, defaults)

			if tc.wantCode != "" {
				var promptErr *masterprompt.Error
				if !errors.As(err, &promptErr) || promptErr.Code != tc.wantCode {
					t.Fatalf("Fallback() error = %v, want code %q", err, tc.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("Fallback() error = %v, want nil", err)
			}
			if doc.Version != masterprompt.Version {
				t.Fatalf("version = %q, want %q", doc.Version, masterprompt.Version)
			}
			if doc.Output.AspectRatio != tc.aspect || doc.StopConditions.AspectRatio != tc.aspect {
				t.Fatalf("aspect = %q/%q, want %q on both", doc.Output.AspectRatio, doc.StopConditions.AspectRatio, tc.aspect)
			}
			if doc.Context.Style.Medium != tc.medium {
				t.Fatalf("medium = %q, want %q", doc.Context.Style.Medium, tc.medium)
			}
			if tc.duration != 0 {
				if doc.Output.DurationS == nil || *doc.Output.DurationS != tc.duration {
					t.Fatalf("duration = %v, want %d", doc.Output.DurationS, tc.duration)
				}
			}
			if tc.wantDetails != "" && !contains(doc.Context.Details, tc.wantDetails) {
				t.Fatalf("details = %v, want it to contain %q", doc.Context.Details, tc.wantDetails)
			}
			if err := masterprompt.Validate(doc); err != nil {
				t.Fatalf("Validate() on the fallback document error = %v, want nil", err)
			}
		})
	}
}

func TestFallback_GivenAValidDocument_ThenCompileAcceptsIt(t *testing.T) {
	doc, err := masterprompt.Fallback("foto produk kopi, aspect 1:1, photo", "image",
		masterprompt.Defaults{AspectRatio: "16:9", Style: "cinematic photo"})
	if err != nil {
		t.Fatalf("Fallback() error = %v, want nil", err)
	}

	compiled, err := masterprompt.Compile(doc)
	if err != nil {
		t.Fatalf("Compile() error = %v, want nil", err)
	}
	if !strings.Contains(compiled.Text, "avoid: ") {
		t.Fatalf("compiled prompt = %q, want the baseline negatives", compiled.Text)
	}
	if !strings.Contains(strings.ToLower(compiled.Text), "kopi") {
		t.Fatalf("compiled prompt = %q, want the user subject kept", compiled.Text)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
