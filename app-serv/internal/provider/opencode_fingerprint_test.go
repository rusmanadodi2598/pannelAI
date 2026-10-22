// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_fingerprint_test.go
// @for       The tool-level conceal rules: exact read/shell names, aliasing a conflicting caller tool, and keeping a canonical declaration verbatim.
// @uses      encoding/json, testing.
// @reason    The upstream compares the description and parameter schema of the declared read/shell tools literally and answers 403 FreeTierError when they differ, so the alias rule that keeps a caller's own tool of the same name is pinned here. The canonical declarations and append rule live in opencode_fingerprint_definitions_test.go, and the body-level rules in opencode_fingerprint_body_test.go, for the AGENTS.md §1.1 budget.
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
