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

// TestClampOpenCodeChatCeiling pins the chat wire's half of the same rule. Before
// it, only the Responses transform bounded a ceiling, so a client ceiling too
// small for the model it addressed was forwarded verbatim and answered with
// nothing but thinking (measured live 2026-10-16: max_tokens 2500 spent 2499
// tokens on reasoning, 0 on the answer).
func TestClampOpenCodeChatCeiling_GivenACeilingUnderTheModelsFloor_ThenItIsRaised(t *testing.T) {
	body := openCodeBodyOf(t, `{"max_tokens":8}`)
	clampOpenCodeChatCeiling(body, registry.Model{MinOutputTokens: 1024})

	if got := string(body["max_tokens"]); got != "1024" {
		t.Fatalf("max_tokens = %s, want the model's floor", got)
	}
}

func TestClampOpenCodeChatCeiling_GivenNoCeiling_ThenNoneIsInvented(t *testing.T) {
	body := openCodeBodyOf(t, `{"model":"opencode/x"}`)
	clampOpenCodeChatCeiling(body, registry.Model{MinOutputTokens: 1024})

	if _, present := body["max_tokens"]; present {
		t.Fatalf("a ceiling was invented where the client sent none: %s", body["max_tokens"])
	}
}

func TestClampOpenCodeChatCeiling_GivenACeilingOverTheWireCap_ThenItIsCut(t *testing.T) {
	body := openCodeBodyOf(t, `{"max_tokens":100000}`)
	clampOpenCodeChatCeiling(body, registry.Model{})

	if got := string(body["max_tokens"]); got != "65536" {
		t.Fatalf("max_tokens = %s, want the wire's own cap", got)
	}
}

func TestClampOpenCodeChatCeiling_GivenACeilingWithinTheBounds_ThenBytesAreUnchanged(t *testing.T) {
	for _, raw := range []string{`{"max_tokens":4000}`, `{"max_tokens":65536}`} {
		body := openCodeBodyOf(t, raw)
		before := string(body["max_tokens"])
		clampOpenCodeChatCeiling(body, registry.Model{MinOutputTokens: 512, ContextLength: 1000000})

		if got := string(body["max_tokens"]); got != before {
			t.Fatalf("max_tokens = %s, want the client's %s untouched", got, before)
		}
	}
}

func TestClampOpenCodeChatCeiling_GivenBothCeilingKeys_ThenTheCompletionKeyWins(t *testing.T) {
	body := openCodeBodyOf(t, `{"max_tokens":4000,"max_completion_tokens":8}`)
	clampOpenCodeChatCeiling(body, registry.Model{MinOutputTokens: 1024})

	if got := string(body["max_completion_tokens"]); got != "1024" {
		t.Fatalf("max_completion_tokens = %s, want it raised: it is the key the upstream reads", got)
	}
	if got := string(body["max_tokens"]); got != "4000" {
		t.Fatalf("max_tokens = %s, want the client's own value left as written", got)
	}
}

// The raise is the only place the gateway writes a bigger number than the client
// sent, so it is the only place the context window can be over-reached. A ceiling
// the client asked for is forwarded even when it is too big to raise to, because
// a starved answer is survivable and a 400 is not.
func TestClampOpenCodeChatCeiling_GivenAFloorOverHalfTheWindow_ThenTheClientNumberStands(t *testing.T) {
	body := openCodeBodyOf(t, `{"max_tokens":512}`)
	clampOpenCodeChatCeiling(body, registry.Model{MinOutputTokens: 200000, ContextLength: 262144})

	if got := string(body["max_tokens"]); got != "512" {
		t.Fatalf("max_tokens = %s, want the client's number rather than a ceiling that earns 400", got)
	}
}

func TestClampOpenCodeChatCeiling_GivenAnUndeclaredWindow_ThenTheRaiseStopsAtTheBlindBound(t *testing.T) {
	raised := openCodeBodyOf(t, `{"max_tokens":8}`)
	clampOpenCodeChatCeiling(raised, registry.Model{MinOutputTokens: openCodeBlindRaiseBound})
	if got := string(raised["max_tokens"]); got != "4096" {
		t.Fatalf("max_tokens = %s, want the blind bound a model with no declared window allows", got)
	}

	stayed := openCodeBodyOf(t, `{"max_tokens":8}`)
	clampOpenCodeChatCeiling(stayed, registry.Model{MinOutputTokens: openCodeBlindRaiseBound + 1})
	if got := string(stayed["max_tokens"]); got != "8" {
		t.Fatalf("max_tokens = %s, want no raise beyond the blind bound", got)
	}
}

func TestClampOpenCodeChatCeiling_GivenADeclaredModelMaximum_ThenTheRaiseStopsThere(t *testing.T) {
	body := openCodeBodyOf(t, `{"max_tokens":8}`)
	clampOpenCodeChatCeiling(body, registry.Model{MinOutputTokens: 8192, MaxOutputTokens: 4096, ContextLength: 1000000})

	if got := string(body["max_tokens"]); got != "8" {
		t.Fatalf("max_tokens = %s, want no raise past the model's own declared maximum", got)
	}
}

func TestClampOpenCodeChatCeiling_GivenACeilingThatIsNotAInteger_ThenItIsLeftAsWritten(t *testing.T) {
	body := openCodeBodyOf(t, `{"max_tokens":"lots"}`)
	clampOpenCodeChatCeiling(body, registry.Model{MinOutputTokens: 512})

	if got := string(body["max_tokens"]); got != `"lots"` {
		t.Fatalf("max_tokens = %s, want the client's malformed value forwarded, not rewritten", got)
	}
}

// The Responses wire already bounded ceilings this way; the two paths must not
// drift on the number the upstream finally sees.
func TestOpenCodeCeiling_BothWiresAgreeOnTheBounds(t *testing.T) {
	chat := openCodeBodyOf(t, `{"max_tokens":1}`)
	clampOpenCodeChatCeiling(chat, registry.Model{MinOutputTokens: 512})
	responses := openCodeBodyOf(t, `{"max_output_tokens":1}`)
	renameOpenCodeCeiling(responses, registry.Model{MinOutputTokens: 512})

	if string(chat["max_tokens"]) != string(responses["max_output_tokens"]) {
		t.Fatalf("chat raised to %s while responses raised to %s, want one floor",
			chat["max_tokens"], responses["max_output_tokens"])
	}
}
