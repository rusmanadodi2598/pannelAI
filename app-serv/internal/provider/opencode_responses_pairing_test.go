// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_pairing_test.go
// @for       The call/output pairing the OpenCode Responses wire demands of a replayed conversation.
// @uses      encoding/json, strings, testing.
// @reason    Claude Code compacts a conversation and interrupts tool turns, which leaves a `function_call` with no answer, or an answer with no call, in the history it replays; /zen/v1/responses reads either as 400 "No tool output found for function call ...". Pairing is decided on the call_id the upstream sees, after the clamp, so these cases pin the post-clamp reading rather than the client's spelling.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

// openCodeWireItem is one input item as the upstream receives it.
type openCodeWireItem struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

// pairedOpenCodeItems runs one fixture input through the corrections and reads
// the items back in the order the wire will see them.
func pairedOpenCodeItems(t *testing.T, input string) []openCodeWireItem {
	t.Helper()
	body := openCodeBodyOf(t, `{"input":`+input+`}`)
	normalizeOpenCodeItems(body)
	var items []openCodeWireItem
	if err := json.Unmarshal(body["input"], &items); err != nil {
		t.Fatalf("reading the paired input back: %v (%s)", err, body["input"])
	}
	return items
}

func TestNormalizeOpenCodeItems_ClosesACallNoOneAnswered(t *testing.T) {
	items := pairedOpenCodeItems(t, `[{"type":"function_call","name":"read","call_id":"c1","arguments":"{}"}]`)
	if len(items) != 2 {
		t.Fatalf("input = %s, want the call plus the answer that closes it", wireItems(items))
	}
	if items[1].Type != "function_call_output" || items[1].CallID != "c1" {
		t.Fatalf("the closing answer is %s/%q, want function_call_output on c1", items[1].Type, items[1].CallID)
	}
	if items[1].Output != "{}" {
		t.Fatalf("the closing answer carries %q, want the empty object the reference writes", items[1].Output)
	}
}

func TestNormalizeOpenCodeItems_DropsAnAnswerNoOneAskedFor(t *testing.T) {
	items := pairedOpenCodeItems(t, `[{"type":"function_call_output","call_id":"c1","output":"hi"}]`)
	if len(items) != 0 {
		t.Fatalf("orphan answer survived: %s", wireItems(items))
	}
}

func TestNormalizeOpenCodeItems_PairsAnAnswerThatArrivesLater(t *testing.T) {
	items := pairedOpenCodeItems(t, `[`+
		`{"type":"function_call","name":"read","call_id":"c1","arguments":"{}"},`+
		`{"type":"message","role":"assistant","content":"thinking"},`+
		`{"type":"function_call_output","call_id":"c1","output":"file"}`+
		`]`)
	if len(items) != 3 {
		t.Fatalf("a call answered twenty items later must not be closed twice: %s", wireItems(items))
	}
	if items[1].Type != "message" || items[2].Type != "function_call_output" {
		t.Fatalf("pairing reordered the conversation: %s", wireItems(items))
	}
}

func TestNormalizeOpenCodeItems_OneAnswerCoversRepeatsOfOneCallID(t *testing.T) {
	items := pairedOpenCodeItems(t, `[`+
		`{"type":"function_call","name":"read","call_id":"c1","arguments":"{}"},`+
		`{"type":"function_call","name":"read","call_id":"c1","arguments":"{}"}`+
		`]`)
	if len(items) != 3 {
		t.Fatalf("two calls on one id got %s, want one closing answer between them", wireItems(items))
	}
}

func TestNormalizeOpenCodeItems_PairsOnTheClampedIDNotTheClients(t *testing.T) {
	// Two ids that differ only past the 64-character bound are one id to the
	// upstream, so pairing has to agree with the upstream and not with the client.
	long := strings.Repeat("x", 67)
	calls := `{"type":"function_call","name":"read","call_id":"` + long + `-aa","arguments":"{}"}`
	answers := `{"type":"function_call_output","call_id":"` + long + `-bb","output":"file"}`
	items := pairedOpenCodeItems(t, `[`+calls+`,`+answers+`]`)
	if len(items) != 2 {
		t.Fatalf("ids that clamp to one value were paired as two: %s", wireItems(items))
	}
}

func TestNormalizeOpenCodeItems_PinsTheIdLessCollapse(t *testing.T) {
	// app-serv maps every absent call_id onto one value, where the reference
	// mints a fresh one per call. A call and an answer that both carry no id
	// therefore pair. Pinned deliberately: changing the fallback is a contract
	// decision, not a pairing fix.
	items := pairedOpenCodeItems(t, `[`+
		`{"type":"function_call","name":"read","call_id":"","arguments":"{}"},`+
		`{"type":"function_call_output","call_id":"","output":"file"}`+
		`]`)
	if len(items) != 2 || items[1].CallID != "call_pannelai" {
		t.Fatalf("id-less items resolved to %s, want them paired on the one fallback id", wireItems(items))
	}
}

func TestNormalizeOpenCodeItems_StillDropsACallWithNoName(t *testing.T) {
	items := pairedOpenCodeItems(t, `[{"type":"function_call","call_id":"c1","arguments":"{}"}]`)
	if len(items) != 0 {
		t.Fatalf("a nameless call survived, and nothing can dispatch it: %s", wireItems(items))
	}
}

func TestNormalizeOpenCodeItems_LeavesOtherItemsAlone(t *testing.T) {
	items := pairedOpenCodeItems(t, `[{"type":"message","role":"user","content":"hello"}]`)
	if len(items) != 1 || items[0].Role != "user" {
		t.Fatalf("a plain message was rewritten: %s", wireItems(items))
	}
}

// wireItems renders a decoded input for a failure message.
func wireItems(items []openCodeWireItem) string {
	encoded, err := json.Marshal(items)
	if err != nil {
		return "(unreadable)"
	}
	return string(encoded)
}
