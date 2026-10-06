// Package provider ports the reference's per-provider request shaping into
// pluggable connectors (SPEC-API-001 §7.4).
//
// @file      internal/provider/codebuddy_body_test.go
// @for       The CodeBuddy outbound body: the leading system turn, typed user blocks, and the reasoning mirror.
// @uses      encoding/json, testing, internal/registry.
// @reason    The vendor answers a plain OpenAI body with `11101 invalid request`, and the reference rebuilds the messages to avoid it (open-sse/executors/codebuddy-intl.js:20-38). The rewrite is the provider's whole shape requirement, so it is pinned here as data: one leading system turn that carries the caller's own system text rather than dropping it, user strings turned into typed blocks, array content left exactly as the translator built it, and reasoning reported the way this service reads it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package provider

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

func codeBuddyConnector() *CodeBuddy {
	return NewCodeBuddy(registry.Provider{
		ID: "codebuddy-intl", Transport: registry.Transport{Format: "openai"},
	})
}

// shapedBody runs the transform and decodes what would go on the wire.
func shapedBody(t *testing.T, body string) map[string]json.RawMessage {
	t.Helper()
	request := &Request{Body: []byte(body)}
	if err := codeBuddyConnector().TransformRequest(request); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(request.Body, &decoded); err != nil {
		t.Fatalf("the shaped body is not a JSON object: %v (%s)", err, request.Body)
	}
	return decoded
}

func shapedMessages(t *testing.T, body string) []map[string]any {
	t.Helper()
	raw, ok := shapedBody(t, body)["messages"]
	if !ok {
		t.Fatal("the shaped body carries no messages")
	}
	var messages []map[string]any
	if err := json.Unmarshal(raw, &messages); err != nil {
		t.Fatalf("messages are not an array: %v", err)
	}
	return messages
}

func TestCodeBuddyBody_ForcesItsOwnLeadingSystemAndKeepsTheCallers(t *testing.T) {
	messages := shapedMessages(t, `{"messages":[
		{"role":"system","content":"Reply in one word."},
		{"role":"developer","content":"Prefer British spelling."},
		{"role":"user","content":"Colour or color?"}]}`)

	if len(messages) != 2 {
		t.Fatalf("messages = %d, want the vendor's system turn and the user turn: %v", len(messages), messages)
	}
	first, _ := messages[0]["content"].(string)
	if messages[0]["role"] != "system" {
		t.Fatalf("messages[0].role = %v, want system", messages[0]["role"])
	}
	if first != "You are CodeBuddy Code.\n\nReply in one word.\n\nPrefer British spelling." {
		t.Fatalf("leading system = %q, want the vendor prompt carrying both caller instructions", first)
	}
	if messages[1]["role"] != "user" {
		t.Fatalf("messages[1].role = %v, want the caller's turn next", messages[1]["role"])
	}
}

func TestCodeBuddyBody_UserStringContentBecomesTypedBlocks(t *testing.T) {
	messages := shapedMessages(t, `{"messages":[{"role":"user","content":"pong"}]}`)
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want the vendor's system turn then the caller's", len(messages))
	}

	blocks, ok := messages[1]["content"].([]any)
	if !ok {
		t.Fatalf("user content = %T, want typed blocks", messages[1]["content"])
	}
	block, _ := blocks[0].(map[string]any)
	if block["type"] != "text" || block["text"] != "pong" {
		t.Fatalf("block = %v, want {type:text,text:pong}", block)
	}
}

func TestCodeBuddyBody_LeavesBuiltContentAndOtherTurnsAlone(t *testing.T) {
	messages := shapedMessages(t, `{"messages":[
		{"role":"user","content":[{"type":"text","text":"what is in this image?"},{"type":"image_url","image_url":{"url":"https://example.test/red.png"}}]},
		{"role":"assistant","content":"a red square"},
		{"role":"tool","tool_call_id":"call_1","content":"ok"}]}`)

	if len(messages) != 4 {
		t.Fatalf("messages = %d, want the vendor's system turn plus all three caller turns", len(messages))
	}
	built, _ := messages[1]["content"].([]any)
	if len(built) != 2 {
		t.Fatalf("user blocks = %v, want the translator's array untouched", messages[1]["content"])
	}
	second, _ := built[1].(map[string]any)
	if second["type"] != "image_url" {
		t.Fatalf("second block = %v, want the image part preserved", second)
	}
	if messages[3]["tool_call_id"] != "call_1" {
		t.Fatalf("tool turn = %v, want its fields unchanged", messages[3])
	}
}

func TestCodeBuddyBody_ReasoningIsMirroredTheWayTheVendorReadsIt(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantEffort  bool
		wantSummary bool
	}{
		{name: "a stated effort is mirrored", body: `{"messages":[{"role":"user","content":"x"}],"reasoning_effort":"high"}`, wantEffort: true, wantSummary: true},
		{name: "none is dropped", body: `{"messages":[{"role":"user","content":"x"}],"reasoning_effort":"none"}`},
		{name: "off is dropped", body: `{"messages":[{"role":"user","content":"x"}],"reasoning_effort":"off"}`},
		{name: "silence adds nothing", body: `{"messages":[{"role":"user","content":"x"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decoded := shapedBody(t, tc.body)
			_, hasEffort := decoded["reasoning_effort"]
			_, hasSummary := decoded["reasoning_summary"]
			if hasEffort != tc.wantEffort {
				t.Fatalf("reasoning_effort present = %v, want %v", hasEffort, tc.wantEffort)
			}
			if hasSummary != tc.wantSummary {
				t.Fatalf("reasoning_summary present = %v, want %v", hasSummary, tc.wantSummary)
			}
			if tc.wantSummary {
				var summary string
				if err := json.Unmarshal(decoded["reasoning_summary"], &summary); err != nil || summary != "auto" {
					t.Fatalf("reasoning_summary = %s, want \"auto\"", decoded["reasoning_summary"])
				}
			}
		})
	}
}

func TestCodeBuddyBody_RefusesABodyItCannotRead(t *testing.T) {
	cases := []string{`[]`, `"text"`, `not json`, ``, `{"messages":"hi"}`, `{"messages":[],"reasoning_effort":7}`}
	for _, body := range cases {
		if err := codeBuddyConnector().TransformRequest(&Request{Body: []byte(body)}); err == nil {
			t.Fatalf("TransformRequest(%q) = nil, want a refusal: a body half rewritten is worse than none", body)
		}
	}
}

func TestCodeBuddyBody_IsByteStableAcrossCalls(t *testing.T) {
	const body = `{"model":"glm-5.2","messages":[{"role":"system","content":"be brief"},{"role":"user","content":"hi"}],"temperature":0.2}`
	first := &Request{Body: []byte(body)}
	second := &Request{Body: []byte(body)}
	if err := codeBuddyConnector().TransformRequest(first); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	if err := codeBuddyConnector().TransformRequest(second); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	if !bytes.Equal(first.Body, second.Body) {
		t.Fatalf("the shaped body differs between identical calls:\n %s\n %s", first.Body, second.Body)
	}
}
