// Package provider ports the reference's per-provider request shaping into
// pluggable connectors (SPEC-API-001 §7.4).
//
// @file      internal/provider/codebuddy_choice_test.go
// @for       The CodeBuddy forced tool choice: what a named function becomes, and what is left exactly as the caller sent it.
// @uses      encoding/json, testing, internal/provider Request.
// @reason    The vendor answers `auto` and `required` and refuses the OpenAI named-function object, so the rewrite is the only way a caller that has already chosen its tool gets an answer. Pinned as data because the harm is silent: a narrowed tool list that dropped the wrong function, or a choice demoted to `auto`, would return a 200 with a tool call the caller did not ask for.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-30
package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

const codeBuddyTwoTools = `[` +
	`{"type":"function","function":{"name":"get_weather","description":"w","parameters":{"type":"object"}}},` +
	`{"type":"function","function":{"name":"lookup","description":"l","parameters":{"type":"object"}}}]`

func TestCodeBuddyToolChoice_NamedFunctionBecomesRequiredWithOneTool(t *testing.T) {
	body := shapedBody(t, `{"messages":[{"role":"user","content":"Weather in Paris."}],`+
		`"tools":`+codeBuddyTwoTools+`,`+
		`"tool_choice":{"type":"function","function":{"name":"get_weather"}}}`)

	var choice string
	if err := json.Unmarshal(body["tool_choice"], &choice); err != nil {
		t.Fatalf("tool_choice = %s, want the string required", body["tool_choice"])
	}
	if choice != "required" {
		t.Fatalf("tool_choice = %q, want required", choice)
	}

	var tools []map[string]json.RawMessage
	if err := json.Unmarshal(body["tools"], &tools); err != nil {
		t.Fatalf("tools are not a list: %v (%s)", err, body["tools"])
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %d, want only the forced one: %s", len(tools), body["tools"])
	}
	if name := toolFunctionName(tools[0]); name != "get_weather" {
		t.Fatalf("the kept tool is %q, want the one the caller named", name)
	}
}

func TestCodeBuddyToolChoice_KeepsTheForcedToolWhole(t *testing.T) {
	nested := `{"type":"function","function":{"name":"lookup","description":"l",` +
		`"parameters":{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}},"strict":true}`
	body := shapedBody(t, `{"messages":[{"role":"user","content":"x"}],"tools":[`+nested+`],`+
		`"tool_choice":{"type":"function","function":{"name":"lookup"}}}`)

	var kept []map[string]json.RawMessage
	if err := json.Unmarshal(body["tools"], &kept); err != nil {
		t.Fatalf("tools do not decode: %v", err)
	}
	if string(kept[0]["function"]) == "" {
		t.Fatal("the tool's function block was lost")
	}
	if _, present := kept[0]["strict"]; !present {
		t.Fatal("a member the caller declared beside function was dropped")
	}
	if !strings.Contains(string(kept[0]["function"]), `"required":["q"]`) {
		t.Fatalf("the tool's parameter schema was narrowed away: %s", kept[0]["function"])
	}
}

func TestCodeBuddyToolChoice_LeavesChoicesTheVendorReads(t *testing.T) {
	for _, choice := range []string{`"auto"`, `"none"`, `"required"`} {
		body := shapedBody(t, `{"messages":[{"role":"user","content":"x"}],"tools":`+
			codeBuddyTwoTools+`,"tool_choice":`+choice+`}`)
		if string(body["tool_choice"]) != choice {
			t.Fatalf("tool_choice = %s, want %s left as sent", body["tool_choice"], choice)
		}
		var tools []map[string]json.RawMessage
		if err := json.Unmarshal(body["tools"], &tools); err != nil {
			t.Fatalf("tools do not decode: %v", err)
		}
		if len(tools) != 2 {
			t.Fatalf("tools = %d, want both kept for a choice the vendor reads", len(tools))
		}
	}
}

func TestCodeBuddyToolChoice_LeavesAChoiceNamingNoDeclaredTool(t *testing.T) {
	// The caller asked to force a tool it never declared. That is refused by the
	// vendor and is the caller's to fix; narrowing the list to nothing, or picking
	// a different tool on its behalf, would both answer a request that was not made.
	body := shapedBody(t, `{"messages":[{"role":"user","content":"x"}],"tools":`+
		codeBuddyTwoTools+`,"tool_choice":{"type":"function","function":{"name":"absent"}}}`)

	var named namedToolChoice
	if err := json.Unmarshal(body["tool_choice"], &named); err != nil {
		t.Fatalf("tool_choice = %s, want the object left as sent", body["tool_choice"])
	}
	if named.Function.Name != "absent" {
		t.Fatalf("tool_choice was rewritten to %q, want the caller's own choice", named.Function.Name)
	}
}

func TestCodeBuddyToolChoice_RunsBesideTheOtherShaping(t *testing.T) {
	// The reasoning mirror and the leading system turn still apply to a body that
	// carries a forced tool, because the three rewrites share one encoded body.
	body := shapedBody(t, `{"messages":[{"role":"system","content":"Terse."},`+
		`{"role":"user","content":"Weather in Paris."}],`+
		`"tools":`+codeBuddyTwoTools+`,`+
		`"tool_choice":{"type":"function","function":{"name":"lookup"}},`+
		`"reasoning_effort":"high"}`)

	if string(body["reasoning_effort"]) != `"high"` {
		t.Fatalf("reasoning_effort = %s, want the caller's own effort kept", body["reasoning_effort"])
	}
	if string(body["reasoning_summary"]) != `"auto"` {
		t.Fatalf("reasoning_summary = %s, want auto mirrored", body["reasoning_summary"])
	}
	var messages []map[string]any
	if err := json.Unmarshal(body["messages"], &messages); err != nil {
		t.Fatalf("messages do not decode: %v", err)
	}
	if messages[0]["role"] != "system" {
		t.Fatal("the leading system turn the vendor requires is missing")
	}
}
