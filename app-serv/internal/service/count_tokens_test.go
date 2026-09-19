// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/count_tokens_test.go
// @for       Table-driven tests for the count_tokens estimate and the rule it
//
//	is derived by.
//
// @uses      encoding/json, internal/schema, strings, testing.
// @reason    TDD.md §2.5 requires boundary coverage, not one sample: the
//
//	rounding boundary is where a wrong implementation hides, and the
//	request-level cases prove the text is summed before it is divided.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestEstimateInputTokens pins the character-to-token rule at its boundaries:
// zero stays zero, and every partial group of four rounds up rather than down.
func TestEstimateInputTokens(t *testing.T) {
	cases := []struct {
		name  string
		chars int
		want  int
	}{
		{"no text at all", 0, 0},
		{"a negative count is treated as none", -7, 0},
		{"one character", 1, 1},
		{"a partial group of three", 3, 1},
		{"exactly one token", 4, 1},
		{"one past the boundary", 5, 2},
		{"two exact tokens", 8, 2},
		{"two tokens and a remainder", 9, 3},
		{"a page of text", 4096, 1024},
		{"the body cap in characters", 8 << 20, 2 << 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := estimateInputTokens(tc.chars); got != tc.want {
				t.Fatalf("estimateInputTokens(%d) = %d, want %d", tc.chars, got, tc.want)
			}
		})
	}
}

// TestTokenCountService_Count drives the use case the way the handler does:
// decode an Anthropic-wire body, then estimate it. The multi-part case is the
// one that catches per-part rounding, which would report two tokens where the
// reference reports one.
func TestTokenCountService_Count(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{
			name: "a bare string body",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`,
			want: 2,
		},
		{
			name: "two parts are summed before dividing",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[{"type":"text","text":"ab"},{"type":"text","text":"cd"}]}]}`,
			want: 1,
		},
		{
			name: "an exact multiple of four",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"abcd"}]}`,
			want: 1,
		},
		{
			name: "no messages at all",
			body: `{"model":"claude-sonnet-4"}`,
			want: 0,
		},
		{
			name: "blocks that carry no text",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"aGk="}}]}]}`,
			want: 0,
		},
		{
			name: "a large body scales linearly",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"` + strings.Repeat("a", 40000) + `"}]}`,
			want: 10000,
		},
	}
	service := NewTokenCountService()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := schema.DecodeCountTokensRequest([]byte(tc.body))
			if err != nil {
				t.Fatalf("DecodeCountTokensRequest() error = %v", err)
			}
			if got := service.Count(req).InputTokens; got != tc.want {
				t.Fatalf("Count().InputTokens = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestCountTokensResponse_Shape pins the wire answer: Anthropic's own shape, one
// field, no extra members a client would have to ignore.
func TestCountTokensResponse_Shape(t *testing.T) {
	encoded, err := json.Marshal(NewTokenCountService().Count(schema.CountTokensRequest{}))
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	if string(encoded) != `{"input_tokens":0}` {
		t.Fatalf("answer = %s, want {\"input_tokens\":0}", encoded)
	}
}
