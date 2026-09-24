// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_fingerprint_test.go
// @for       The tool-name rule the OpenCode free tier enforces: every declared
// //
//
//	name is unique, and the decoy tools the gate reads are lowercase.
//
// @uses      testing, internal/registry.
// @reason    Measured against the live upstream on 2026-09-24: a body declaring
//
//	the same tool name twice is refused 400 for any name, not only for the
//	decoy quartet, and a body whose only bash/read are capitalised is
//	refused 403 because the gate matches the lowercase spelling. Both
//	failures surface as a provider error with no field named, so the
//	connector has to hold the rule rather than forward it. The rules live
//	in their own file because opencode_body_test.go already covers the
//	field-name rules, and AGENTS.md §1.1 asks for the split.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestOpenCode_TransformDeduplicatesToolNames pins the duplicate rule: the
// upstream refuses a body that declares one name twice with 400, whichever name
// it is, so the connector keeps the first declaration and drops the repeats.
//
// The first declaration wins rather than the last because a client that repeats
// a name is stating the same tool twice, and reordering the client's own list to
// prefer a later copy would change a body the upstream never asked to have
// changed.
func TestOpenCode_TransformDeduplicatesToolNames(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name  string
		tools string
		want  []string
	}{
		{
			name:  "an exact repeat of the decoy",
			tools: `[{"type":"function","function":{"name":"bash"}},{"type":"function","function":{"name":"bash"}}]`,
			want:  []string{"bash", "read"},
		},
		{
			name:  "an exact repeat of a client tool",
			tools: `[{"type":"function","function":{"name":"glob"}},{"type":"function","function":{"name":"glob"}}]`,
			want:  []string{"glob", "bash", "read"},
		},
		{
			name:  "a repeat of read after the decoys were appended",
			tools: `[{"type":"function","function":{"name":"read"}},{"type":"function","function":{"name":"read"}}]`,
			want:  []string{"read", "bash"},
		},
		{
			name:  "the same name in both declaration shapes",
			tools: `[{"type":"function","function":{"name":"bash"}},{"type":"function","name":"bash"}]`,
			want:  []string{"bash", "read"},
		},
		{
			name:  "no duplicates at all",
			tools: `[{"type":"function","function":{"name":"glob"}},{"type":"function","function":{"name":"bash"}},{"type":"function","function":{"name":"read"}}]`,
			want:  []string{"glob", "bash", "read"},
		},
		{
			name:  "a distinct name that only differs by case is kept",
			tools: `[{"type":"function","function":{"name":"Bash"}},{"type":"function","function":{"name":"bash"}}]`,
			want:  []string{"Bash", "bash", "read"},
		},
		{
			name:  "an empty list still gains both decoys",
			tools: `[]`,
			want:  []string{"bash", "read"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{
				Model: registry.Model{ID: "big-pickle"},
				Body:  []byte(`{"model":"big-pickle","messages":[],"tools":` + tc.tools + `}`),
			}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			got := toolNames(t, decodeBody(t, request.Body))
			if len(got) != len(tc.want) {
				t.Fatalf("tool names = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("tool names = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestOpenCode_TransformKeepsTheLowercaseGateSpelling pins the other half of the
// measured rule: the gate matches `bash` and `read` in lowercase, so a body whose
// only spelling of them is capitalised is refused 403 even though the duplicate
// rule accepts the mixture. The connector therefore has to make sure the
// lowercase pair is present without renaming what the client declared, because
// renaming would change the name the client dispatches on when the model calls it.
func TestOpenCode_TransformKeepsTheLowercaseGateSpelling(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name  string
		tools string
		want  []string
	}{
		{
			name:  "only capitalised spellings",
			tools: `[{"type":"function","function":{"name":"Bash"}},{"type":"function","function":{"name":"Read"}}]`,
			want:  []string{"Bash", "Read", "bash", "read"},
		},
		{
			name:  "one capitalised and one lowercase",
			tools: `[{"type":"function","function":{"name":"Bash"}},{"type":"function","function":{"name":"read"}}]`,
			want:  []string{"Bash", "read", "bash"},
		},
		{
			name:  "already lowercase, so nothing is added",
			tools: `[{"type":"function","function":{"name":"bash"}},{"type":"function","function":{"name":"read"}}]`,
			want:  []string{"bash", "read"},
		},
		{
			name:  "a client tool that merely starts with the same letters",
			tools: `[{"type":"function","function":{"name":"bashful"}}]`,
			want:  []string{"bashful", "bash", "read"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{
				Model: registry.Model{ID: "big-pickle"},
				Body:  []byte(`{"model":"big-pickle","messages":[],"tools":` + tc.tools + `}`),
			}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			got := toolNames(t, decodeBody(t, request.Body))
			if len(got) != len(tc.want) {
				t.Fatalf("tool names = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("tool names = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
