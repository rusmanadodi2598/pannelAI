// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_fingerprint_definitions_test.go
// @for       The canonical read/shell declarations OpenCode compares literally, and the rule that appends whichever one is missing.
// @uses      encoding/json, testing.
// @reason    A declaration that differs in wording or required members is what the upstream answers with 403 FreeTierError, so the verbatim text is pinned here rather than only the alias behaviour. The append rule is the second half: a caller that already declared one canonical tool gains only the other.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package provider

import (
	"encoding/json"
	"testing"
)

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

// TestAppendOpenCodeFingerprintTools pins which canonical tools are appended:
// the reference's order, no duplicate of one the caller already declared, and an
// aliased caller tool does not satisfy the canonical name.
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
