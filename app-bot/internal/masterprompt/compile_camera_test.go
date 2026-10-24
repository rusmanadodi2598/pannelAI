// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/compile_camera_test.go
// @for       Token dedup and camera-move rendering in the compiled prompt.
// @uses      strings, testing.
// @reason    These two behaviors pin the token model and the video camera line, so they live beside the golden tests that pin section order.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt_test

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/masterprompt"
)

func TestCompile_GivenRepeatedTokens_ThenOnlyTheFirstOneStays(t *testing.T) {
	cases := []struct {
		name     string
		palette  []string
		details  []string
		negative []string
		phrase   string
		present  string
	}{
		{
			name:    "the same phrase in different cases stays once",
			palette: []string{"warm orange", "Warm Orange", "teal shadow"},
			details: []string{"warm orange", "motion blur in the wheels"},
			phrase:  "warm orange",
			present: "motion blur in the wheels",
		},
		{
			name:    "repeated details keep only the first slot",
			palette: []string{"warm orange", "teal shadow"},
			details: []string{"teal shadow", "teal shadow", "motion blur in the wheels"},
			phrase:  "teal shadow",
			present: "motion blur in the wheels",
		},
		{
			name:    "a lighting phrase repeated in details is dropped",
			palette: []string{"warm orange"},
			details: []string{"soft shadows", "motion blur in the wheels"},
			phrase:  "soft shadows",
			present: "motion blur in the wheels",
		},
		{
			name:     "repeated negatives are emitted once",
			palette:  []string{"warm orange"},
			details:  []string{"motion blur in the wheels"},
			negative: []string{"watermark", "watermark"},
			phrase:   "watermark",
			present:  "watermark",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeImage(t)
			doc.Context.Style.Palette = tc.palette
			doc.Context.Details = tc.details
			if tc.negative != nil {
				doc.StopConditions.Negative = tc.negative
			}

			compiled, err := masterprompt.Compile(doc)
			if err != nil {
				t.Fatalf("Compile() error = %v, want nil", err)
			}
			if got := strings.Count(strings.ToLower(compiled.Text), tc.phrase); got != 1 {
				t.Fatalf("%q appears %d times, want 1", tc.phrase, got)
			}
			if !strings.Contains(compiled.Text, tc.present) {
				t.Fatalf("compiled prompt = %q, must still contain %q", compiled.Text, tc.present)
			}
		})
	}
}

func TestCompile_GivenCameraMoveDetails_ThenTheMoveIsWrittenSpaced(t *testing.T) {
	cases := []struct {
		name    string
		details []string
		want    string
	}{
		{
			name:    "an underscored move is written with a space",
			details: []string{"pan_left", "steam rising"},
			want:    "camera: pan left",
		},
		{
			name:    "an orbit detail names the orbit camera",
			details: []string{"orbit", "steam rising"},
			want:    "camera: orbit",
		},
		{
			name:    "a capitalized move is recognized case freely",
			details: []string{"PULL_OUT", "steam rising"},
			want:    "camera: pull out",
		},
		{
			name:    "a move written with a space is still recognized",
			details: []string{"push in", "steam rising"},
			want:    "camera: push in",
		},
		{
			name:    "no camera move keeps the static default",
			details: []string{"steam rising"},
			want:    "camera: static",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeVideo(t)
			doc.Context.Details = tc.details

			compiled, err := masterprompt.Compile(doc)
			if err != nil {
				t.Fatalf("Compile() error = %v, want nil", err)
			}
			if !strings.Contains(compiled.Text, tc.want) {
				t.Fatalf("compiled prompt = %q, want it to contain %q", compiled.Text, tc.want)
			}
		})
	}
}
