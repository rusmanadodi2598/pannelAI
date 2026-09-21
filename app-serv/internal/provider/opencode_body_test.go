// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_body_test.go
// @for       The OpenCode Free request shape: forced streaming, the decoy tools,
//
//	the tool_choice rule, and the Responses field names.
//
// @uses      testing, encoding/json, internal/registry.
// @reason    The free tier answers 403 to a body that does not carry a streamed
//
//	request and both decoy tools, and 400 to a Responses body that
//	names max_tokens or a tool_choice other than auto. Those are upstream
//	rules the gateway must apply on the way out, so each one is pinned
//	against the decoded body rather than against a string the test would
//	have to keep in step with the encoder.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// decodeBody decodes an outbound payload into a generic object, so a test reads
// the members the upstream reads without depending on the encoder's field order.
func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding the transformed body: %v (raw=%s)", err, raw)
	}
	return decoded
}

// toolNames reads the tool names present in either wire shape the free tier
// accepts: the Responses form names the function on the tool itself, and the chat
// form nests it under `function`.
func toolNames(t *testing.T, body map[string]any) []string {
	t.Helper()
	raw, ok := body["tools"]
	if !ok {
		return nil
	}
	entries, ok := raw.([]any)
	if !ok {
		t.Fatalf("tools = %T, want an array", raw)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		tool, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := tool["name"].(string); ok && name != "" {
			names = append(names, name)
			continue
		}
		if fn, ok := tool["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok && name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// contains reports whether a name appears in a list.
func contains(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

// TestOpenCode_TransformForcesStreaming pins the first half of the gate: the free
// tier answers 403 to a non-streaming request on both wires, so the outbound body
// always streams regardless of what the client asked for.
func TestOpenCode_TransformForcesStreaming(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name   string
		model  registry.Model
		stream bool
		body   string
	}{
		{name: "chat, client asked for none", model: registry.Model{ID: "big-pickle"},
			body: `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"stream":false}`},
		{name: "chat, client asked for it", model: registry.Model{ID: "big-pickle"}, stream: true,
			body: `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"stream":true}`},
		{name: "chat, stream absent", model: registry.Model{ID: "big-pickle"},
			body: `{"model":"big-pickle","messages":[{"role":"user","content":"q"}]}`},
		{name: "responses, client asked for none", model: registry.Model{ID: "muse-spark-1.3-contributor-free", TargetFormat: "openai-responses"},
			body: `{"model":"muse-spark-1.3-contributor-free","input":[],"stream":false}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{Model: tc.model, Body: []byte(tc.body), Stream: tc.stream}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			if streamed, ok := decodeBody(t, request.Body)["stream"].(bool); !ok || !streamed {
				t.Fatalf("stream = %v, want true in the outbound body", decodeBody(t, request.Body)["stream"])
			}
			if !request.Stream {
				t.Fatal("Request.Stream = false, want true so the transport reads a stream")
			}
		})
	}
}

// TestOpenCode_TransformAddsBothDecoyTools pins the second half of the gate: the
// upstream requires both `bash` and `read` in the payload, and a client's own
// tools must survive alongside them because the model still has to be able to
// call what the client declared.
func TestOpenCode_TransformAddsBothDecoyTools(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name  string
		model registry.Model
		body  string
	}{
		{
			name:  "chat, no tools at all",
			model: registry.Model{ID: "big-pickle"},
			body:  `{"model":"big-pickle","messages":[{"role":"user","content":"q"}]}`,
		},
		{
			name:  "chat, an empty tool list",
			model: registry.Model{ID: "big-pickle"},
			body:  `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"tools":[]}`,
		},
		{
			name:  "chat, only bash",
			model: registry.Model{ID: "big-pickle"},
			body:  `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"tools":[{"type":"function","function":{"name":"bash","parameters":{"type":"object"}}}]}`,
		},
		{
			name:  "chat, only read",
			model: registry.Model{ID: "big-pickle"},
			body:  `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"tools":[{"type":"function","function":{"name":"read","parameters":{"type":"object"}}}]}`,
		},
		{
			name:  "chat, the client's own tool",
			model: registry.Model{ID: "big-pickle"},
			body:  `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"tools":[{"type":"function","function":{"name":"my_tool","description":"mine","parameters":{"type":"object"}}}]}`,
		},
		{
			name:  "responses, no tools at all",
			model: registry.Model{ID: "muse-spark-1.3-contributor-free", TargetFormat: "openai-responses"},
			body:  `{"model":"muse-spark-1.3-contributor-free","input":[]}`,
		},
		{
			name:  "responses, the client's own tool",
			model: registry.Model{ID: "muse-spark-1.2-contributor-free", TargetFormat: "openai-responses"},
			body:  `{"model":"muse-spark-1.2-contributor-free","input":[],"tools":[{"type":"function","name":"my_tool","description":"mine","parameters":{"type":"object"}}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{Model: tc.model, Body: []byte(tc.body)}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			names := toolNames(t, decodeBody(t, request.Body))
			for _, want := range []string{"bash", "read"} {
				if !contains(names, want) {
					t.Fatalf("tools = %v, want %q present", names, want)
				}
			}
		})
	}

	// A client's own tool must survive the injection, and a decoy the client
	// already declared must not be duplicated.
	t.Run("the client's own tool survives", func(t *testing.T) {
		body := `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],` +
			`"tools":[{"type":"function","function":{"name":"my_tool","parameters":{"type":"object"}}}]}`
		request := Request{Model: registry.Model{ID: "big-pickle"}, Body: []byte(body)}
		if err := connector.TransformRequest(&request); err != nil {
			t.Fatalf("TransformRequest() error = %v", err)
		}
		names := toolNames(t, decodeBody(t, request.Body))
		if !contains(names, "my_tool") {
			t.Fatalf("tools = %v, want the client's own tool kept", names)
		}
		if len(names) != 3 {
			t.Fatalf("tools = %v, want the client's tool plus one of each decoy", names)
		}
	})
}
