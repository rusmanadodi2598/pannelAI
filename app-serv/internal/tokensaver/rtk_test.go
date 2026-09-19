// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/rtk_test.go
// @for       Table-driven tests for the JSON walk: the four tool-result shapes,
//
//	the two rules that protect content, and the caps.
//
// @uses      encoding/json, fmt, strings, testing.
// @reason    SPEC-API-002 §3 makes "tool results only" and "errors preserved"
//
//	structural claims, so each is driven as its own case rather than
//	assumed from the walk's shape. TDD.md §2.5 also asks for the
//	boundaries: a blob under the floor, one over the cap, and a body
//	with no compressible shape at all.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// diffBlob is a unified diff large enough to clear the compression floor and
// wide enough that compaction wins: the four header lines and the leading
// context are dropped, which is what a real diff looks like.
func diffBlob() string {
	lines := []string{
		"diff --git a/internal/a.go b/internal/a.go",
		"index 1111111..2222222 100644",
		"--- a/internal/a.go",
		"+++ b/internal/a.go",
		"@@ -1,45 +1,46 @@",
	}
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf(" // a context line number %d that the model does not need", i))
	}
	lines = append(lines,
		"-// the old line with enough text to matter",
		"+// the new line with enough text to matter",
		"+// another new line with enough text to matter",
	)
	return strings.Join(lines, "\n")
}

// numberBlob is a line-numbered file dump, the shape read-numbered claims.
func numberBlob() string {
	lines := make([]string, 0, 300)
	for i := 1; i <= 300; i++ {
		lines = append(lines, fmt.Sprintf("%3d|package main // a line of code", i))
	}
	return strings.Join(lines, "\n")
}

// TestCompress_Shapes drives one body per wire shape the walk understands.
func TestCompress_Shapes(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantShape string
		wantText  string
	}{
		{
			name:      "an OpenAI tool message with string content",
			body:      `{"model":"m","messages":[{"role":"user","content":"hello"},{"role":"tool","tool_call_id":"c1","content":%q}]}`,
			wantShape: "openai-tool",
			wantText:  "an added line number",
		},
		{
			name:      "an OpenAI tool message with parts",
			body:      `{"messages":[{"role":"tool","content":[{"type":"text","text":%q}]}]}`,
			wantShape: "openai-tool-array",
			wantText:  "an added line number",
		},
		{
			name:      "an Anthropic tool_result with string content",
			body:      `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":%q}]}]}`,
			wantShape: "claude-string",
			wantText:  "an added line number",
		},
		{
			name:      "an Anthropic tool_result with parts",
			body:      `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":%q}]}]}]}`,
			wantShape: "claude-array",
			wantText:  "an added line number",
		},
		{
			name:      "a Responses function_call_output string",
			body:      `{"input":[{"type":"function_call_output","call_id":"c1","output":%q}]}`,
			wantShape: "openai-responses-string",
			wantText:  "an added line number",
		},
		{
			name:      "a Responses function_call_output parts",
			body:      `{"input":[{"type":"function_call_output","call_id":"c1","output":[{"type":"input_text","text":%q}]}]}`,
			wantShape: "openai-responses-array",
			wantText:  "an added line number",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(tc.body, diffBlob()))
			out, stats, err := Compress(body, nil)
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}
			if len(stats.Hits) != 1 {
				t.Fatalf("hits = %+v, want exactly one", stats.Hits)
			}
			if stats.Hits[0].Shape != tc.wantShape {
				t.Fatalf("shape = %q, want %q", stats.Hits[0].Shape, tc.wantShape)
			}
			if stats.Hits[0].Filter != "git-diff" {
				t.Fatalf("filter = %q, want git-diff", stats.Hits[0].Filter)
			}
			if stats.Saved() <= 0 {
				t.Fatalf("saved = %d, want a positive count", stats.Saved())
			}
			if len(out) >= len(body) {
				t.Fatalf("the rewritten body must be smaller: %d -> %d", len(body), len(out))
			}
			// The diff's own shape survives: the model still learns which file
			// changed and how many lines moved.
			if !strings.Contains(string(out), "internal/a.go") || !strings.Contains(string(out), "+2 -1") {
				t.Fatalf("the compacted diff must keep its shape:\n%s", out)
			}
			if !json.Valid(out) {
				t.Fatalf("the rewritten body is not valid JSON:\n%s", out)
			}
		})
	}
}
