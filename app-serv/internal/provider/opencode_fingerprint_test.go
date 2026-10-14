// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_fingerprint_test.go
// @for       The tool-level fingerprint rules: exact read/shell names, aliasing a conflicting caller tool, and the canonical declarations that are appended.
// @uses      encoding/json, testing.
// @reason    The upstream compares the description and parameter schema of the declared read/shell tools literally and answers 403 FreeTierError when they differ, so both the canonical definitions and the alias rule that keeps a caller's own tool of the same name are pinned here. The body-level rules (choice retarget, history rename, placeholder note) are pinned in opencode_fingerprint_body_test.go for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"encoding/json"
	"testing"
)

// flatTools builds flat Responses-shaped declarations with just a name.
func flatTools(names ...string) []json.RawMessage {
	tools := make([]json.RawMessage, 0, len(names))
	for _, name := range names {
		tools = append(tools, json.RawMessage(`{"type":"function","name":`+string(mustRawString(name))+`}`))
	}
	return tools
}

// chatTools builds chat-shaped declarations with just a name.
func chatTools(names ...string) []json.RawMessage {
	tools := make([]json.RawMessage, 0, len(names))
	for _, name := range names {
		tools = append(tools, json.RawMessage(`{"type":"function","function":{"name":`+string(mustRawString(name))+`}}`))
	}
	return tools
}

// toolNamesOf reads the names a tool list declares in either shape.
func toolNamesOf(tools []json.RawMessage) []string {
	names := make([]string, 0, len(tools))
	for _, raw := range tools {
		names = append(names, openCodeToolName(raw))
	}
	return names
}

func TestOpenCodeFingerprintKey_MatchesExactNamesOnly(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"read", "read"},
		{" shell ", "shell"},
		{"Read", ""},
		{"bash", ""},
		{"glob", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := openCodeFingerprintKey(tc.name); got != tc.want {
			t.Fatalf("openCodeFingerprintKey(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestConcealOpenCodeFingerprintTools(t *testing.T) {
	cases := []struct {
		name      string
		tools     []json.RawMessage
		wantNames []string
		wantAlias map[string]string
	}{
		{
			name:      "a caller tool on a fingerprint name with its own definition is aliased",
			tools:     flatTools("read", "Edit"),
			wantNames: []string{"read_client", "Edit"},
			wantAlias: map[string]string{"read_client": "read"},
		},
		{
			name:      "a taken alias steps aside",
			tools:     flatTools("read", "read_client"),
			wantNames: []string{"read_client2", "read_client"},
			wantAlias: map[string]string{"read_client2": "read"},
		},
		{
			name:      "case variants are caller tools and travel verbatim",
			tools:     chatTools("Read", "Bash"),
			wantNames: []string{"Read", "Bash"},
			wantAlias: map[string]string{},
		},
		{
			name:      "a repeated fingerprint name is dropped after the first",
			tools:     chatTools("shell", "shell"),
			wantNames: []string{"shell_client"},
			wantAlias: map[string]string{"shell_client": "shell"},
		},
		{
			name:      "a repeated caller name is dropped after the first",
			tools:     chatTools("glob", "glob"),
			wantNames: []string{"glob"},
			wantAlias: map[string]string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kept, aliases := concealOpenCodeFingerprintTools(tc.tools)
			got := toolNamesOf(kept)
			if len(got) != len(tc.wantNames) {
				t.Fatalf("names = %v, want %v", got, tc.wantNames)
			}
			for index := range tc.wantNames {
				if got[index] != tc.wantNames[index] {
					t.Fatalf("names = %v, want %v", got, tc.wantNames)
				}
			}
			if len(aliases) != len(tc.wantAlias) {
				t.Fatalf("aliases = %v, want %v", aliases, tc.wantAlias)
			}
			for alias, original := range tc.wantAlias {
				if aliases[alias] != original {
					t.Fatalf("aliases = %v, want %s -> %s", aliases, alias, original)
				}
			}
		})
	}
}

// TestConcealOpenCodeFingerprintTools_KeepsTheCanonicalDefinition pins the other
// half of the rule: a caller tool that already carries OpenCode's exact
// declaration is left exactly as written, so no alias is needed and nothing is
// appended beside it.
func TestConcealOpenCodeFingerprintTools_KeepsTheCanonicalDefinition(t *testing.T) {
	canonical := []json.RawMessage{
		openCodeFingerprintTool("read", true),
		openCodeFingerprintTool("shell", false),
	}
	kept, aliases := concealOpenCodeFingerprintTools(canonical)
	if len(aliases) != 0 {
		t.Fatalf("aliases = %v, want none for canonical definitions", aliases)
	}
	if got := toolNamesOf(kept); len(got) != 2 || got[0] != "read" || got[1] != "shell" {
		t.Fatalf("names = %v, want the canonical pair kept verbatim", got)
	}
	if string(kept[0]) != string(canonical[0]) {
		t.Fatalf("the canonical declaration was rewritten:\n %s\nwant\n %s", kept[0], canonical[0])
	}
}

// TestOpenCodeJSONEqual_ComparesByValue pins the canonical-schema comparison:
// the same members in any order and formatting count as OpenCode's own, because
// a JSON API compares the decoded value (and the standard marshaller re-escapes
// `&`, `<` and `>` inside the declaration the connector appends, which a byte
// comparison would misread even against itself).
func TestOpenCodeJSONEqual_ComparesByValue(t *testing.T) {
	if !openCodeJSONEqual(json.RawMessage(`{"a":1,"b":2}`), json.RawMessage(`{ "b": 2, "a": 1 }`)) {
		t.Fatal("the same members in a different order and formatting must compare equal")
	}
	if openCodeJSONEqual(json.RawMessage(`{"a":1,"b":2}`), json.RawMessage(`{"a":1,"b":3}`)) {
		t.Fatal("a different member value must compare unequal")
	}
}

// TestOpenCodeFingerprintTool_CarriesTheVerbatimDefinition pins the declarations
// the upstream compares against: the reference's own wording, the required
// members, and the wire's own shape.
func TestOpenCodeFingerprintTool_CarriesTheVerbatimDefinition(t *testing.T) {
	flat := decodeRaw(t, openCodeFingerprintTool("read", true))
	if flat["strict"] != false {
		t.Fatalf("flat strict = %v, want false", flat["strict"])
	}
	if _, nested := flat["function"]; nested {
		t.Fatal("the flat declaration nests its function, want it named on the tool")
	}
	parameters, _ := flat["parameters"].(map[string]any)
	if got, _ := parameters["required"].([]any); len(got) != 1 || got[0] != "path" {
		t.Fatalf("read required = %v, want [path]", parameters["required"])
	}
	if properties, _ := parameters["properties"].(map[string]any); len(properties) != 3 {
		t.Fatalf("read properties = %v, want path, offset, and limit", parameters["properties"])
	}

	nested := decodeRaw(t, openCodeFingerprintTool("shell", false))
	function, ok := nested["function"].(map[string]any)
	if !ok {
		t.Fatalf("chat declaration = %v, want a nested function", nested)
	}
	if function["description"] != openCodeFingerprintDefinitions["shell"].description {
		t.Fatal("the shell description is not the verbatim reference text")
	}
	shellParameters, _ := function["parameters"].(map[string]any)
	if got, _ := shellParameters["required"].([]any); len(got) != 1 || got[0] != "command" {
		t.Fatalf("shell required = %v, want [command]", shellParameters["required"])
	}
}

func TestAppendOpenCodeFingerprintTools(t *testing.T) {
	cases := []struct {
		name        string
		tools       []json.RawMessage
		responses   bool
		wantNames   []string
		wantAppends []string
	}{
		{
			name:        "an empty list gains both, in the reference's order",
			tools:       nil,
			responses:   true,
			wantNames:   []string{"read", "shell"},
			wantAppends: []string{"read", "shell"},
		},
		{
			name:        "a declared canonical read is not duplicated",
			tools:       []json.RawMessage{openCodeFingerprintTool("read", false)},
			responses:   false,
			wantNames:   []string{"read", "shell"},
			wantAppends: []string{"shell"},
		},
		{
			name:        "an aliased caller tool leaves the canonical name missing",
			tools:       flatTools("read_client"),
			responses:   true,
			wantNames:   []string{"read_client", "read", "shell"},
			wantAppends: []string{"read", "shell"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tools, appended := appendOpenCodeFingerprintTools(tc.tools, tc.responses)
			if got := toolNamesOf(tools); len(got) != len(tc.wantNames) {
				t.Fatalf("names = %v, want %v", got, tc.wantNames)
			} else {
				for index := range tc.wantNames {
					if got[index] != tc.wantNames[index] {
						t.Fatalf("names = %v, want %v", got, tc.wantNames)
					}
				}
			}
			if len(appended) != len(tc.wantAppends) {
				t.Fatalf("appended = %v, want %v", appended, tc.wantAppends)
			}
			for index := range tc.wantAppends {
				if appended[index] != tc.wantAppends[index] {
					t.Fatalf("appended = %v, want %v", appended, tc.wantAppends)
				}
			}
		})
	}
}

// decodeRaw decodes one encoded member for the shape checks above.
func decodeRaw(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	return decoded
}
