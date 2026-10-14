// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_fingerprint_body_test.go
// @for       The body-level fingerprint rules: the choice retarget, the history rename, the placeholder note, and the never-default-to-none rule.
// @uses      encoding/json, strings, testing.
// @reason    A concealed caller tool has to stay one name across the whole request: an explicit tool_choice follows it to its alias, and a prior-turn call is renamed with it, or the model reads its own history as calls to OpenCode's tool of the same name. The note is the reference's measured wording, and the chat wire must never gain a defaulted "none", which upstream answers with 403 FreeTierError. They sit apart from the tool-level rules for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

const openCodeNoteSuffix = "Do not call the `read` or `shell` tools; they are placeholders that do nothing. Use the other tools you have been given instead."

// TestApplyOpenCodeFingerprintTools_AliasesAcrossTheWholeBody pins the pass on
// the Responses wire: the caller's own `read` travels as `read_client`, its
// explicit choice and its prior call follow it, and the canonical declarations
// are appended beside it.
func TestApplyOpenCodeFingerprintTools_AliasesAcrossTheWholeBody(t *testing.T) {
	body := openCodeBodyOf(t, `{
		"input":[
			{"type":"function_call","name":"read","call_id":"c1","arguments":"{}"},
			{"type":"function_call_output","call_id":"c1","output":"file"}
		],
		"tools":[{"type":"function","name":"read","description":"mine","parameters":{"type":"object"}}],
		"tool_choice":{"type":"function","name":"read"},
		"instructions":"Be brief."
	}`)
	applyOpenCodeFingerprintTools(body, true)

	if got := toolNamesIn(t, body["tools"]); !equalNames(got, []string{"read_client", "read", "shell"}) {
		t.Fatalf("tools = %v, want the alias plus the canonical pair", got)
	}
	choice := string(body["tool_choice"])
	if !strings.Contains(choice, `"name":"read_client"`) {
		t.Fatalf("tool_choice = %s, want it retargeted to the alias", choice)
	}
	input := string(body["input"])
	if !strings.Contains(input, `"name":"read_client"`) {
		t.Fatalf("input = %s, want the prior call renamed with its declaration", input)
	}
	instructions := stringMember(body, "instructions")
	if !strings.HasPrefix(instructions, "Be brief.\n\n") || !strings.Contains(instructions, openCodeNoteSuffix) {
		t.Fatalf("instructions = %q, want the note appended to the caller's own", instructions)
	}
}

// TestApplyOpenCodeFingerprintTools_RenamesChatHistoryCalls pins the same rename
// on the chat wire, where the prior call lives in a message's tool_calls.
func TestApplyOpenCodeFingerprintTools_RenamesChatHistoryCalls(t *testing.T) {
	body := openCodeBodyOf(t, `{
		"messages":[
			{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"shell","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"c1","content":"done"}
		],
		"tools":[{"type":"function","function":{"name":"shell","description":"mine","parameters":{"type":"object"}}}]
	}`)
	applyOpenCodeFingerprintTools(body, false)

	if got := string(body["messages"]); !strings.Contains(got, `"name":"shell_client"`) {
		t.Fatalf("messages = %s, want the prior call renamed to the alias", got)
	}
	if got := toolNamesIn(t, body["tools"]); !equalNames(got, []string{"shell_client", "read", "shell"}) {
		t.Fatalf("tools = %v, want the alias plus the canonical pair", got)
	}
	messages := decodeItems(t, body["messages"])
	first := messages[0]
	if first["role"] != "system" || !strings.Contains(first["content"].(string), openCodeNoteSuffix) {
		t.Fatalf("first message = %v, want the placeholder note in a system message", first)
	}
}

// TestApplyOpenCodeFingerprintTools_NeverDefaultsToNone pins the rule the
// upstream enforces with 403 FreeTierError: the Responses wire keeps its
// explicit auto default, and the chat wire gains no default at all.
func TestApplyOpenCodeFingerprintTools_NeverDefaultsToNone(t *testing.T) {
	responses := openCodeBodyOf(t, `{"input":[]}`)
	applyOpenCodeFingerprintTools(responses, true)
	if got := string(responses["tool_choice"]); got != `"auto"` {
		t.Fatalf("responses tool_choice = %s, want auto", got)
	}

	chat := openCodeBodyOf(t, `{"messages":[{"role":"user","content":"hi"}]}`)
	applyOpenCodeFingerprintTools(chat, false)
	if got, present := chat["tool_choice"]; present {
		t.Fatalf("chat tool_choice = %s, want none written", got)
	}
}

// TestNoteOpenCodePlaceholderTools pins the three prompt shapes: the Responses
// instructions, the OpenAI chat system message, and the Claude-shaped body the
// note must leave alone because it carries `system` instead.
func TestNoteOpenCodePlaceholderTools(t *testing.T) {
	t.Run("responses instructions are appended to", func(t *testing.T) {
		body := openCodeBodyOf(t, `{"instructions":"Be brief."}`)
		noteOpenCodePlaceholderTools(body, true, []string{"read", "shell"})
		if got := stringMember(body, "instructions"); got != "Be brief.\n\n"+openCodeNoteSuffix {
			t.Fatalf("instructions = %q", got)
		}
	})

	t.Run("a chat system message is appended to", func(t *testing.T) {
		body := openCodeBodyOf(t, `{"messages":[{"role":"system","content":"Be brief."},{"role":"user","content":"hi"}]}`)
		noteOpenCodePlaceholderTools(body, false, []string{"read", "shell"})
		messages := decodeItems(t, body["messages"])
		if len(messages) != 2 {
			t.Fatalf("messages = %s, want the list kept", body["messages"])
		}
		if got, _ := messages[0]["content"].(string); got != "Be brief.\n\n"+openCodeNoteSuffix {
			t.Fatalf("system content = %q", got)
		}
	})

	t.Run("a chat body with no system message gains one", func(t *testing.T) {
		body := openCodeBodyOf(t, `{"messages":[{"role":"user","content":"hi"}]}`)
		noteOpenCodePlaceholderTools(body, false, []string{"read", "shell"})
		messages := decodeItems(t, body["messages"])
		if len(messages) != 2 {
			t.Fatalf("messages = %s, want a system message prepended", body["messages"])
		}
		first := messages[0]
		if first["role"] != "system" || first["content"] != openCodeNoteSuffix {
			t.Fatalf("first message = %v, want the note", first)
		}
	})

	t.Run("a Claude-shaped body is left alone", func(t *testing.T) {
		body := openCodeBodyOf(t, `{"system":"Be brief.","messages":[{"role":"user","content":"hi"}]}`)
		noteOpenCodePlaceholderTools(body, false, []string{"read", "shell"})
		messages := decodeItems(t, body["messages"])
		if len(messages) != 1 {
			t.Fatalf("messages = %s, want the Claude-shaped list untouched", body["messages"])
		}
	})

	t.Run("one appended tool reads in the singular", func(t *testing.T) {
		if got := openCodePlaceholderNote([]string{"read"}); !strings.Contains(got, "the `read` tool; it is a placeholder") {
			t.Fatalf("note = %q, want the singular wording", got)
		}
	})
}

// TestOpenCode_TransformRunsTheFingerprintPassOnBothWires runs the whole
// connector on both wires, which is the shape a real request takes.
func TestOpenCode_TransformRunsTheFingerprintPassOnBothWires(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	t.Run("chat with no tools", func(t *testing.T) {
		request := Request{Model: registry.Model{ID: "big-pickle"}, Body: []byte(`{"model":"big-pickle","messages":[{"role":"user","content":"q"}]}`)}
		if err := connector.TransformRequest(&request); err != nil {
			t.Fatalf("TransformRequest() error = %v", err)
		}
		body := decodeBody(t, request.Body)
		if got := toolNames(t, body); !equalNames(got, []string{"read", "shell"}) {
			t.Fatalf("tools = %v, want the canonical pair", got)
		}
		if _, present := body["tool_choice"]; present {
			t.Fatalf("tool_choice = %v, want none written on the chat wire", body["tool_choice"])
		}
		messages, _ := body["messages"].([]any)
		first, _ := messages[0].(map[string]any)
		if first["role"] != "system" || !strings.Contains(first["content"].(string), openCodeNoteSuffix) {
			t.Fatalf("first message = %v, want the placeholder note", first)
		}
	})

	t.Run("responses with no tools", func(t *testing.T) {
		request := Request{
			Model: registry.Model{ID: "muse-spark-1.3-contributor-free", TargetFormat: registry.FormatOpenAIResponses},
			Body:  []byte(`{"model":"muse-spark-1.3-contributor-free","input":[]}`),
		}
		if err := connector.TransformRequest(&request); err != nil {
			t.Fatalf("TransformRequest() error = %v", err)
		}
		body := decodeBody(t, request.Body)
		if got := toolNames(t, body); !equalNames(got, []string{"read", "shell"}) {
			t.Fatalf("tools = %v, want the canonical pair", got)
		}
		tools, _ := body["tools"].([]any)
		first, _ := tools[0].(map[string]any)
		if first["strict"] != false {
			t.Fatalf("flat strict = %v, want false", first["strict"])
		}
		if body["tool_choice"] != "auto" {
			t.Fatalf("tool_choice = %v, want auto", body["tool_choice"])
		}
		if instructions, _ := body["instructions"].(string); !strings.Contains(instructions, openCodeNoteSuffix) {
			t.Fatalf("instructions = %q, want the note", body["instructions"])
		}
	})
}

// decodeItems decodes an encoded tool or message list for the checks above.
func decodeItems(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	return items
}

// toolNamesIn reads the names a decoded tool list declares in either shape.
func toolNamesIn(t *testing.T, raw []byte) []string {
	t.Helper()
	tools := decodeItems(t, raw)
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if name, ok := tool["name"].(string); ok && name != "" {
			names = append(names, name)
			continue
		}
		if function, ok := tool["function"].(map[string]any); ok {
			if name, ok := function["name"].(string); ok && name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// equalNames compares two name lists element by element.
func equalNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
