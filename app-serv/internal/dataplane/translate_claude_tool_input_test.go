// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_claude_tool_input_test.go
// @for       The parameter-name corrections applied to a tool call answering on the Anthropic wire.
// @uses      testing.
// @reason    Every rule here exists because a model invented a spelling: `cmd` for Bash, `path` for Read, `query` for Grep. Claude Code rejects the call before running it and reports the rejection as a broken tool. The goldens pin what actually leaves the process, including the reference's asymmetry that only Bash's aliases are retired while every other tool keeps both spellings.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"encoding/json"
	"testing"
)

// claudeArgAlias is one tool's arguments as a model sent them, and what the
// client is handed after the names are corrected. The result is a re-encoded
// object, so its members come back in sorted order and the goldens say so.
type claudeArgAlias struct {
	tool        string
	args        string
	want        string
	verbatim    bool
	wantBothSet bool
}

func TestNormalizeClaudeToolInput(t *testing.T) {
	cases := []claudeArgAlias{
		{tool: "Bash", args: `{"cmd":"ls -l"}`, want: `{"command":"ls -l"}`},
		{tool: "Bash", args: `{"script":"echo hi"}`, want: `{"command":"echo hi"}`},
		{tool: "Bash", args: `{"command_line":"ls"}`, want: `{"command":"ls"}`},
		{tool: "Bash", args: `{"commandLine":"ls"}`, want: `{"command":"ls"}`},
		// `input` and `code` are read as a command but kept as members, because
		// the reference retires only the four spellings it deletes.
		{tool: "Bash", args: `{"input":"ls"}`, want: `{"command":"ls","input":"ls"}`, wantBothSet: true},
		{tool: "Bash", args: `{"code":"ls"}`, want: `{"code":"ls","command":"ls"}`, wantBothSet: true},
		{
			tool: "Bash",
			args: `{"cmd":"a","script":"b","command_line":"c","commandLine":"d","code":"e","input":"f"}`,
			want: `{"code":"e","command":"a","input":"f"}`,
		},
		{
			tool: "Bash", args: `{"command":"real","cmd":"alias"}`, want: `{"command":"real"}`,
		},
		{tool: "execute_command", args: `{"cmd":"ls"}`, want: `{"command":"ls"}`},

		// Read, Write, Edit, Glob and Grep get the real name added; the alias
		// stays. Pinning this is the point of the case list, it is what the
		// reference ships, and a future "cleanup" that deletes them is a change
		// of contract, not a bug fix.
		{tool: "Read", args: `{"path":"/etc/hosts"}`, want: `{"file_path":"/etc/hosts","path":"/etc/hosts"}`, wantBothSet: true},
		{tool: "Read", args: `{"file":"/x"}`, want: `{"file":"/x","file_path":"/x"}`, wantBothSet: true},
		{tool: "Read", args: `{"filePath":"/x"}`, want: `{"filePath":"/x","file_path":"/x"}`, wantBothSet: true},
		{tool: "Read", args: `{"filename":"/x"}`, want: `{"file_path":"/x","filename":"/x"}`, wantBothSet: true},
		{tool: "Read", args: `{"path":"/x","limit":"5000"}`, want: `{"file_path":"/x","limit":2000,"path":"/x"}`},
		{tool: "Read", args: `{"limit":"0"}`, want: `{}`},
		{tool: "Read", args: `{"offset":"-5"}`, want: `{"offset":0}`},
		{tool: "Read", args: `{"limit":500,"offset":10}`, want: `{"limit":500,"offset":10}`},
		{
			tool: "Edit",
			args: `{"file":"/x","old_text":"a","new_text":"b"}`,
			want: `{"file":"/x","file_path":"/x","new_string":"b","new_text":"b","old_string":"a","old_text":"a"}`,
		},
		{
			tool: "MultiEdit",
			args: `{"filepath":"/x","search":"a","replace":"b"}`,
			want: `{"file_path":"/x","filepath":"/x","new_string":"b","old_string":"a","replace":"b","search":"a"}`,
		},
		{
			tool: "Write",
			args: `{"path":"/x","text":"hello"}`,
			want: `{"content":"hello","file_path":"/x","path":"/x","text":"hello"}`,
		},
		{
			tool: "Glob",
			args: `{"query":"**/*.go","dir":"/src"}`,
			want: `{"dir":"/src","path":"/src","pattern":"**/*.go","query":"**/*.go"}`,
		},
		{tool: "Grep", args: `{"regex":"todo"}`, want: `{"pattern":"todo","regex":"todo"}`, wantBothSet: true},
		{
			tool: "Grep", args: `{"pattern":"real","glob":"alias"}`, want: `{"glob":"alias","pattern":"real"}`,
		},
		{tool: "proxy_Bash", args: `{"cmd":"ls"}`, want: `{"command":"ls"}`},

		{tool: "Database", args: `{"query":"select 1"}`, want: `{"query":"select 1"}`, verbatim: true},
		{tool: "Bash", args: `[1,2]`, want: `[1,2]`, verbatim: true},
		{tool: "Bash", args: `not json`, want: `not json`, verbatim: true},
		{tool: "Bash", args: `{}`, want: `{}`},
	}
	for _, tc := range cases {
		name := tc.tool + " " + tc.args
		t.Run(name, func(t *testing.T) {
			got := string(normalizeClaudeToolInput(tc.tool, []byte(tc.args)))
			if got != tc.want {
				t.Fatalf("normalizeClaudeToolInput(%s, %s) = %s, want %s", tc.tool, tc.args, got, tc.want)
			}
			if tc.verbatim && got != tc.args {
				t.Fatalf("%s was rewritten into %s, want it passed through untouched", tc.args, got)
			}
			if !tc.verbatim && !tc.wantBothSet && tc.args != tc.want && got == tc.args {
				t.Fatalf("nothing was renamed: %s came back as %s", tc.args, got)
			}
		})
	}
}

// TestNormalizeClaudeToolInput_NeverInventsAValue keeps the correction honest: a
// parameter the model never mentioned is not filled in, and a value that is not
// text is not coerced into one.
func TestNormalizeClaudeToolInput_NeverInventsAValue(t *testing.T) {
	cases := []struct {
		tool string
		args string
	}{
		{"Bash", `{}`},
		{"Bash", `{"timeout":30}`},
		{"Read", `{"cmd":12}`},
		{"Read", `{"path":null}`},
		{"Read", `{"path":{"nested":1}}`},
		{"Grep", `{"pattern":"keep","query":"keep"}`},
	}
	for _, tc := range cases {
		t.Run(tc.tool+" "+tc.args, func(t *testing.T) {
			got := normalizeClaudeToolInput(tc.tool, []byte(tc.args))
			var before, after map[string]json.RawMessage
			if json.Unmarshal([]byte(tc.args), &before) != nil {
				return
			}
			if json.Unmarshal(got, &after) != nil {
				t.Fatalf("the corrected input %s is not an object", got)
			}
			for name, value := range before {
				if string(after[name]) != string(value) {
					t.Fatalf("%s rewrote %s from %s to %s", tc.tool, name, value, after[name])
				}
			}
			for name := range after {
				if _, existed := before[name]; !existed {
					t.Fatalf("%s invented the member %s out of %s", tc.tool, name, tc.args)
				}
			}
		})
	}
}
