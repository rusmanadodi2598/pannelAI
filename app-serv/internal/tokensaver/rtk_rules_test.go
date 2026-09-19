// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/rtk_rules_test.go
// @for       The RTK pass's protective rules and its caps, driven as cases.
// @uses      fmt, strings, testing.
// @reason    SPEC-API-002 §3 makes "tool results only" and "errors preserved"
//
//	structural claims, so each is its own case rather than something
//	assumed from the walk's shape. TDD.md §2.5 also asks for the
//	boundaries: a blob under the floor, one over the cap, and a filter
//	outside the allowlist.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestCompress_NeverTouchesWhatItShouldNot covers the two protective rules and
// the pass-through cases.
func TestCompress_NeverTouchesWhatItShouldNot(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "a user message that looks like a diff",
			body: fmt.Sprintf(`{"messages":[{"role":"user","content":%q}]}`, diffBlob()),
		},
		{
			name: "a tool_result that reports an error",
			body: fmt.Sprintf(`{"messages":[{"role":"user","content":[{"type":"tool_result","is_error":true,"content":%q}]}]}`, diffBlob()),
		},
		{
			name: "a blob under the floor",
			body: `{"messages":[{"role":"tool","content":"short output"}]}`,
		},
		{
			name: "a blob nothing claims",
			body: fmt.Sprintf(`{"messages":[{"role":"tool","content":%q}]}`, strings.Repeat("x", 900)),
		},
		{
			name: "a body with no messages or input",
			body: `{"model":"m","prompt":"hello"}`,
		},
		{
			name: "an empty messages array",
			body: `{"messages":[]}`,
		},
		{
			name: "a null message",
			body: `{"messages":[null,{"role":"tool","content":"short"}]}`,
		},
		{
			name: "a message whose content is a number",
			body: `{"messages":[{"role":"tool","content":42}]}`,
		},
		{
			name: "a tool_result with no content",
			body: `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1"}]}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(tc.body)
			out, stats, err := Compress(body, nil)
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}
			if string(out) != tc.body {
				t.Fatalf("the body must be returned byte for byte:\n got %s\nwant %s", out, tc.body)
			}
			if len(stats.Hits) != 0 {
				t.Fatalf("hits = %+v, want none", stats.Hits)
			}
		})
	}
}

// TestCompress_RespectsTheCap pins the 10 MiB ceiling: a diff-shaped blob over
// the cap is passed through rather than filtered, which is what keeps a large
// file dump intact.
func TestCompress_RespectsTheCap(t *testing.T) {
	huge := "diff --git a/big.go b/big.go\n@@ -1,1 +1,100000 @@\n" + strings.Repeat("+added line\n", (RawCap/12)+10)
	body := []byte(fmt.Sprintf(`{"messages":[{"role":"tool","content":%q}]}`, huge))
	out, stats, err := Compress(body, nil)
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if !bytes.Equal(out, body) {
		t.Fatal("a blob over the cap must be passed through")
	}
	if len(stats.Hits) != 0 {
		t.Fatalf("hits = %+v, want none", stats.Hits)
	}
	if stats.BytesBefore != len(huge) || stats.BytesAfter != len(huge) {
		t.Fatalf("the cap must still be counted: %+v", stats)
	}
}

// TestCompress_RespectsTheAllowlist pins the configured list on the walk path.
func TestCompress_RespectsTheAllowlist(t *testing.T) {
	body := []byte(fmt.Sprintf(`{"messages":[{"role":"tool","content":%q}]}`, diffBlob()))
	out, stats, err := Compress(body, []string{"ls", "tree"})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if !bytes.Equal(out, body) || len(stats.Hits) != 0 {
		t.Fatalf("a filter outside the allowlist must not run: %+v", stats.Hits)
	}

	out, stats, err = Compress(body, []string{"git-diff"})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if len(stats.Hits) != 1 {
		t.Fatalf("an allowed filter must run: %+v", stats.Hits)
	}
	if len(out) >= len(body) {
		t.Fatalf("the rewritten body must be smaller: %d -> %d", len(body), len(out))
	}
}

// TestCompress_KeepsUnmodelledMembers pins the reason the walk is JSON-level:
// every member the gateway does not model survives the rewrite, and the text
// members it does not touch stay byte for byte.
func TestCompress_KeepsUnmodelledMembers(t *testing.T) {
	body := []byte(fmt.Sprintf(
		`{"model":"claude-x","temperature":0.7,"metadata":{"user_id":"u1"},"tools":[{"name":"bash"}],"messages":[{"role":"user","content":"keep <this> & that"},{"role":"tool","content":%q}]}`,
		numberBlob()))
	out, stats, err := Compress(body, nil)
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if len(stats.Hits) != 1 || stats.Hits[0].Filter != "read-numbered" {
		t.Fatalf("hits = %+v, want one read-numbered hit", stats.Hits)
	}
	text := string(out)
	for _, want := range []string{`"temperature":0.7`, `"metadata":{"user_id":"u1"}`, `"tools":[{"name":"bash"}]`, "keep <this> & that", "... +120 lines truncated (file continues)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the rewritten body must contain %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, `\u003c`) || strings.Contains(text, `\u0026`) {
		t.Fatalf("the rewrite must not HTML-escape the content:\n%s", text)
	}
}

// TestCompress_RejectsABodyThatIsNotAnObject pins the error path the caller
// fails open on.
func TestCompress_RejectsABodyThatIsNotAnObject(t *testing.T) {
	for _, body := range []string{"", "[1,2,3]", "not json at all", `"a string"`} {
		if body == "" {
			continue
		}
		if _, _, err := Compress([]byte(body), nil); err == nil {
			t.Fatalf("Compress(%q) = nil error, want a rejection", body)
		}
	}
	if out, _, err := Compress(nil, nil); err != nil || len(out) != 0 {
		t.Fatalf("an empty body must pass through: %q, %v", out, err)
	}
}
