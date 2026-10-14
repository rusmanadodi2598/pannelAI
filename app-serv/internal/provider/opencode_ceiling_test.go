// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_ceiling_test.go
// @for       The output ceiling bounds the OpenCode Responses wire accepts.
// @uses      strconv, testing, internal/registry.
// @reason    The wire refuses a ceiling below sixteen and refuses the request outright once the ceiling plus the input passes the model's context, which a client asking for the whole window of a 1M-context combo does. The floor was measured and pinned in September; the ceiling is the half that was missing, and both belong to one clamp.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package provider

import (
	"strconv"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

func TestRenameOpenCodeCeiling_BoundsBetweenFloorAndCap(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		model   registry.Model
		want    string
		wantSet bool
	}{
		{"a ceiling above the cap is cut to it", `{"max_output_tokens":100000}`, registry.Model{}, "65536", true},
		{"a one-token probe is raised to the floor", `{"max_output_tokens":1}`, registry.Model{}, "16", true},
		{"a zero ceiling is raised to the floor", `{"max_output_tokens":0}`, registry.Model{}, "16", true},
		{"a ceiling inside the bounds is left alone", `{"max_output_tokens":4000}`, registry.Model{}, "4000", true},
		{"a chat ceiling is renamed then cut", `{"max_tokens":200000}`, registry.Model{}, "65536", true},
		{"the completion ceiling is renamed then cut", `{"max_completion_tokens":99999}`, registry.Model{}, "65536", true},
		{"a model floor is still honoured under the cap", `{"max_output_tokens":8}`, registry.Model{MinOutputTokens: 512}, "512", true},
		{"a floor above the cap resolves to the cap", `{"max_output_tokens":8}`, registry.Model{MinOutputTokens: 70000}, "65536", true},
		{"no ceiling stays no ceiling", `{"model":"oc/muse"}`, registry.Model{}, "", false},
		{"a ceiling that is not a number is left as written", `{"max_output_tokens":"lots"}`, registry.Model{}, `"lots"`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := openCodeBodyOf(t, tc.body)
			renameOpenCodeCeiling(body, tc.model)

			got, present := body["max_output_tokens"]
			if present != tc.wantSet {
				t.Fatalf("max_output_tokens present = %v (%s), want %v", present, got, tc.wantSet)
			}
			if !present {
				return
			}
			if string(got) != tc.want {
				t.Fatalf("max_output_tokens = %s, want %s", got, tc.want)
			}
			if _, leaked := body["max_tokens"]; leaked {
				t.Fatal("the chat-shaped ceiling survived the rename")
			}
			if _, leaked := body["max_completion_tokens"]; leaked {
				t.Fatal("the completion-shaped ceiling survived the rename")
			}
		})
	}
}

func TestOpenCodeMaxOutputTokens_BoundIsTheWireOwn(t *testing.T) {
	if openCodeMaxOutputTokens < openCodeMinOutputTokens {
		t.Fatalf("the ceiling %d is below the floor %d", openCodeMaxOutputTokens, openCodeMinOutputTokens)
	}
	if got := strconv.Itoa(openCodeMaxOutputTokens); got != "65536" {
		t.Fatalf("the cap is %s, want the reference's 65536", got)
	}
}
