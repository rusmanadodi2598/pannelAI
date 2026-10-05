// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_messages_test.go
// @for       The message reshaping and the token ceiling the agent payload is built from.
//
// @uses      encoding/json, strings, testing.
// @reason    These are the two rules with no vendor call in them: which turn the
//
//	endpoint wants lifted out of the history, and what the model is allowed
//	to write. They are table-driven so each client spelling is a named case
//	rather than a claim about all of them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNormalizeQoderMessages pins the reshaping the payload depends on, as a table so
// each client spelling is a named case.
func TestNormalizeQoderMessages(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantSystem string
		wantLast   string
		wantCount  int
	}{
		{
			name: "system turns join with a blank line", in: `[{"role":"system","content":"a"},{"role":"system","content":"b"},{"role":"user","content":"u"}]`,
			wantSystem: "a\n\nb", wantLast: "u", wantCount: 1,
		},
		{
			name: "a text array becomes a string", in: `[{"role":"user","content":[{"type":"text","text":"x"},{"type":"text","text":"y"}]}]`,
			wantSystem: "", wantLast: "x\ny", wantCount: 1,
		},
		{
			name: "an image turn keeps its structure", in: `[{"role":"user","content":[{"type":"text","text":"x"},{"type":"image_url","image_url":{"url":"https://img/y"}}]}]`,
			wantSystem: "", wantLast: "x", wantCount: 1,
		},
		{
			name: "a missing role is served as a user turn", in: `[{"content":"bare"}]`,
			wantSystem: "", wantLast: "bare", wantCount: 1,
		},
		{
			name: "no turns is no system text", in: `[]`, wantSystem: "", wantLast: "", wantCount: 0,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var in []qoderMessage
			if err := json.Unmarshal([]byte(testCase.in), &in); err != nil {
				t.Fatalf("decoding the fixture: %v", err)
			}
			messages, system := normalizeQoderMessages(in)
			if len(messages) != testCase.wantCount {
				t.Fatalf("messages = %d, want %d", len(messages), testCase.wantCount)
			}
			if system != testCase.wantSystem {
				t.Fatalf("system = %q, want %q", system, testCase.wantSystem)
			}
			if got := lastQoderUserText(messages); got != testCase.wantLast {
				t.Fatalf("last user text = %q, want %q", got, testCase.wantLast)
			}
		})
	}
}

// TestNormalizeQoderMessagesKeepsAnImageTurnRaw pins the one case that must not be
// flattened: dropping the array would drop the image with it.
func TestNormalizeQoderMessagesKeepsAnImageTurnRaw(t *testing.T) {
	var in []qoderMessage
	body := `[{"role":"user","content":[{"type":"text","text":"lihat"},{"type":"image_url","image_url":{"url":"https://img/y"}}]}]`
	if err := json.Unmarshal([]byte(body), &in); err != nil {
		t.Fatalf("decoding the fixture: %v", err)
	}

	messages, _ := normalizeQoderMessages(in)
	if !strings.Contains(string(messages[0].Content), "image_url") {
		t.Fatalf("content = %s, want the image part kept", messages[0].Content)
	}
}

// TestQoderMaxTokens pins the ceiling rule on its own: the model's stated output
// limit, or the default, lowered by whatever smaller amount the client asked for.
func TestQoderMaxTokens(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		incoming qoderOpenAIRequest
		want     int
	}{
		{"the model's own limit", `{"max_output_tokens":16384}`, qoderOpenAIRequest{}, 16384},
		{"a client asking for less", `{"max_output_tokens":16384}`, qoderOpenAIRequest{MaxTokens: 512}, 512},
		{"a client asking for more keeps the limit", `{"max_output_tokens":16384}`, qoderOpenAIRequest{MaxTokens: 99999}, 16384},
		{"the responses-style field too", `{"max_output_tokens":16384}`, qoderOpenAIRequest{MaxCompletionTokens: 256}, 256},
		{"no stated limit uses the default", `{}`, qoderOpenAIRequest{}, qoderDefaultMaxTokens},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := qoderMaxTokens(json.RawMessage(testCase.config), testCase.incoming); got != testCase.want {
				t.Fatalf("max_tokens = %d, want %d", got, testCase.want)
			}
		})
	}
}

// TestQoderForcesStream pins the declaration the core reads: this endpoint only
// answers a stream, so a client that asked for one JSON body is served from one.
func TestQoderForcesStream(t *testing.T) {
	connector, _, _ := newQoderStubVendor(t, qoderCatalogFixture)
	if !connector.ForcesStream() {
		t.Fatal("the connector claims a non-streaming answer it does not have")
	}
}
