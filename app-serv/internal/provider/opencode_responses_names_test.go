// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_names_test.go
// @for       The tool-name alphabet and tool_choice membership the OpenCode Responses wire enforces.
// @uses      encoding/json, strings, testing, internal/registry.
// @reason    /zen/v1/responses reads a tool name as `^[a-zA-Z0-9_-]+$`, so an MCP name spelled with `:` or `.` is a 400 before the model is consulted. Renaming the declaration without checking `tool_choice` against the renamed set would trade that 400 for another one, so both halves are pinned here, and pinned to agree with each other.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

func TestSanitizeOpenCodeToolName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"an MCP colon name", "mcp:fs:read", "mcp_fs_read"},
		{"a dotted name", "read.file", "read_file"},
		{"a legal name is untouched", "get_time-zone2", "get_time-zone2"},
		{"a space is not a name character", "list files", "list_files"},
		{"a non-ascii name", "readé", "read_"},
		{"a name past the bound is renamed then cut", strings.Repeat("a:", 100), strings.Repeat("a_", 64)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeOpenCodeToolName(tc.in)
			if got != tc.want {
				t.Fatalf("sanitizeOpenCodeToolName(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if len(got) > openCodeMaxToolNameLen {
				t.Fatalf("name %q is %d characters, past the bound of %d", got, len(got), openCodeMaxToolNameLen)
			}
		})
	}
}

func TestNormalizeOpenCodeTools_RenamesTheDeclarationTheWireReads(t *testing.T) {
	body := openCodeBodyOf(t, `{"tools":[{"type":"function","function":{"name":"mcp:fs:read","description":"read a file","parameters":{"type":"object"}}}]}`)
	normalizeOpenCodeTools(body)

	want := `[{"description":"read a file","name":"mcp_fs_read","parameters":{"properties":{},"type":"object"},"type":"function"}]`
	if got := string(body["tools"]); got != want {
		t.Fatalf("tools =\n %s\nwant\n %s", got, want)
	}
}

func TestNormalizeOpenCodeItems_RenamesTheCallToMatchItsDeclaration(t *testing.T) {
	body := openCodeBodyOf(t, `{"input":[{"type":"function_call","name":"mcp:fs:read","call_id":"c1","arguments":"{}"}]}`)
	normalizeOpenCodeItems(body)

	if got := string(body["input"]); !strings.Contains(got, `"name":"mcp_fs_read"`) {
		t.Fatalf("the call kept an unspellable name: %s", got)
	}
}

func TestNormalizeOpenCodeTools_DropsAChoiceThatNoLongerNamesATool(t *testing.T) {
	cases := []struct {
		name     string
		choice   string
		wantKept bool
	}{
		{"the choice names the spelling the client sent", `"mcp:fs:read"`, false},
		{"the choice names the tool after it was renamed", `"mcp_fs_read"`, true},
		{"the choice is auto", `"auto"`, true},
		{"the choice is a function object without a name", `{"type":"function"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := openCodeBodyOf(t, `{"tools":[{"name":"mcp:fs:read"}],"tool_choice":`+choiceMember(tc.choice)+`}`)
			normalizeOpenCodeTools(body)
			_, kept := body["tool_choice"]
			if kept != tc.wantKept {
				t.Fatalf("tool_choice %s kept = %v, want %v", tc.choice, kept, tc.wantKept)
			}
		})
	}
}

// choiceMember wraps a tool name as the object form the wire reads, and passes a
// bare string through as the string form.
func choiceMember(choice string) string {
	if strings.HasPrefix(choice, `"`) {
		if choice == `"auto"` {
			return choice
		}
		return `{"type":"function","name":` + choice + `}`
	}
	return choice
}

func TestTransformOpenCodeResponses_KeepsOneSpellingAcrossToolsAndItems(t *testing.T) {
	body := openCodeBodyOf(t, `{"tools":[{"name":"mcp:fs:read"}],"input":[{"type":"function_call","name":"mcp:fs:read","call_id":"c1"}]}`)
	transformOpenCodeResponses(body, registry.Model{}, registry.Provider{})

	tools, items := string(body["tools"]), string(body["input"])
	if !strings.Contains(tools, `"mcp_fs_read"`) || !strings.Contains(items, `"mcp_fs_read"`) {
		t.Fatalf("declaration and call disagree on spelling:\n %s\n %s", tools, items)
	}
}

// openCodeBodyOf decodes a fixture body, failing the test on anything that is
// not the object shape the connector reads.
func openCodeBodyOf(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	body, ok := decodeOpenCodeBody([]byte(raw))
	if !ok {
		t.Fatalf("fixture is not a JSON object: %s", raw)
	}
	return body
}
