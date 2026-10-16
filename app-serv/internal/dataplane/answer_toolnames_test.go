// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/answer_toolnames_test.go
// @for       The non-streamed half of the tool-name restore: every upstream wire, the collision cases the alias scheme produces, and the byte-exact passthrough.
// @uses      strings, testing.
// @reason    The live failure this seam exists for came back as `name: "read_client"` on a caller tool the caller had declared as `read`, so the assertions are about the name the caller reads, in each wire the upstream can answer with, and about the rule that nothing changes when nothing was renamed. The split-name accumulation is pinned in answer_toolnames_stream_test.go.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"bytes"
	"strings"
	"testing"
)

// toolCallNames reads every tool call's function name out of a chat payload,
// in the order the payload reports them.
func toolCallNames(t *testing.T, raw []byte) []string {
	t.Helper()
	chunk, ok := decodeObject(raw)
	if !ok {
		t.Fatalf("payload did not decode: %s", raw)
	}
	choices, _ := arrayField(chunk, "choices")
	names := make([]string, 0, 2)
	for _, choiceRaw := range choices {
		choice, _ := decodeObject(choiceRaw)
		for _, holderName := range []string{"message", "delta"} {
			holder, ok := objectField(choice, holderName)
			if !ok {
				continue
			}
			calls, _ := arrayField(holder, "tool_calls")
			for _, callRaw := range calls {
				call, _ := decodeObject(callRaw)
				function, ok := objectField(call, "function")
				if !ok {
					continue
				}
				names = append(names, stringField(function, "name"))
			}
		}
	}
	if len(names) == 0 {
		t.Fatalf("no tool call found in %s", raw)
	}
	return names
}

// TestRestoreToolNamesNonStreamed pins the seam on the chat wire: the upstream
// answers naming the alias the request sent, and the caller reads back the
// name it declared. The collision case is the scheme's own: a taken alias steps
// aside (`read_client2`), and a caller tool that merely shares an alias's
// prefix is not the alias and stays as it is.
func TestRestoreToolNamesNonStreamed(t *testing.T) {
	cases := []struct {
		name    string
		aliases map[string]string
		body    string
		want    []string
	}{
		{
			name:    "a concealed read is restored",
			aliases: map[string]string{"read_client": "read"},
			body:    `{"choices":[{"index":0,"message":{"tool_calls":[{"id":"c1","type":"function","function":{"name":"read_client","arguments":"{\"path\":\"/tmp/x\"}"}}]},"finish_reason":"tool_calls"}]}`,
			want:    []string{"read"},
		},
		{
			name:    "a concealed shell is restored",
			aliases: map[string]string{"shell_client": "shell"},
			body:    `{"choices":[{"index":0,"message":{"tool_calls":[{"id":"c1","function":{"name":"shell_client","arguments":"{}"}}]}}]}`,
			want:    []string{"shell"},
		},
		{
			name:    "both fingerprint names, each under its own collision",
			aliases: map[string]string{"read_client2": "read", "shell_client": "shell"},
			body: `{"choices":[{"index":0,"message":{"tool_calls":[` +
				`{"id":"c1","function":{"name":"read_client2"}},` +
				`{"id":"c2","function":{"name":"shell_client"}},` +
				`{"id":"c3","function":{"name":"read_client"}}` +
				`]}}]}`,
			want: []string{"read", "shell", "read_client"},
		},
		{
			name:    "a name the caller declared itself is left alone",
			aliases: map[string]string{"read_client": "read"},
			body:    `{"choices":[{"index":0,"message":{"tool_calls":[{"id":"c1","function":{"name":"Edit","arguments":"{}"}}]}}]}`,
			want:    []string{"Edit"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toolCallNames(t, restoreToolNames([]byte(tc.body), tc.aliases))
			if len(got) != len(tc.want) {
				t.Fatalf("names = %v, want %v", got, tc.want)
			}
			for index := range tc.want {
				if got[index] != tc.want[index] {
					t.Fatalf("names = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestRestoreToolNamesResponsesAndClaudeShapes pins the other two upstream
// wires the connector can answer on: a Responses function_call item (streamed
// and whole-body, including the response object a terminal event carries), and
// an Anthropic tool_use block (streamed and non-streamed).
func TestRestoreToolNamesResponsesAndClaudeShapes(t *testing.T) {
	aliases := map[string]string{"read_client": "read"}
	cases := []struct {
		name string
		body string
	}{
		{
			name: "a Responses output item",
			body: `{"type":"response.output_item.added","item":{"type":"function_call","name":"read_client","call_id":"c1"}}`,
		},
		{
			name: "a Responses whole output list",
			body: `{"id":"resp_1","output":[{"type":"function_call","name":"read_client","call_id":"c1"}]}`,
		},
		{
			name: "the response object a terminal event carries",
			body: `{"type":"response.completed","response":{"id":"resp_1","output":[{"type":"function_call","name":"read_client","call_id":"c1"}]}}`,
		},
		{
			name: "a Claude content block start",
			body: `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"read_client"}}`,
		},
		{
			name: "a Claude non-streamed message",
			body: `{"type":"message","content":[{"type":"tool_use","id":"t1","name":"read_client","input":{}}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(restoreToolNames([]byte(tc.body), aliases))
			if !strings.Contains(got, `"name":"read"`) {
				t.Fatalf("body = %s, want the caller's read restored", got)
			}
			if strings.Contains(got, "read_client") {
				t.Fatalf("body = %s, want the alias gone", got)
			}
		})
	}
}

// TestRestoreToolNamesPassthrough pins the rule that keeps every other answer
// byte-identical: no alias map, an empty one, or a body whose names were never
// aliased is handed back exactly as it arrived.
func TestRestoreToolNamesPassthrough(t *testing.T) {
	body := []byte(`{"choices":[{"index":0,"message":{"tool_calls":[{"id":"c1","function":{"name":"Edit","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)

	if got := restoreToolNames(body, nil); !bytes.Equal(got, body) {
		t.Fatalf("nil aliases: got %s, want the original bytes", got)
	}
	if got := restoreToolNames(body, map[string]string{}); !bytes.Equal(got, body) {
		t.Fatalf("empty aliases: got %s, want the original bytes", got)
	}
	if got := restoreToolNames(body, map[string]string{"read_client": "read"}); !bytes.Equal(got, body) {
		t.Fatalf("no matching name: got %s, want the original bytes", got)
	}
}
