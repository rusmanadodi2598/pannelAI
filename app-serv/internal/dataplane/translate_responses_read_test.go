// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_read_test.go
// @for       Table-driven tests for reading the parts of a Responses answer.
// @uses      testing.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and an
//
//	upstream may emit item kinds the gateway does not model or parts it
//	cannot carry. Each reader has to skip what it cannot use without
//	failing the answer, since the rest of it is still what the client
//	asked for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "testing"

// TestDecodeResponsesAnswer_SkipsUnknownItems pins that an item type the
// translator has no reader for is ignored rather than failing the answer: a
// provider may emit item kinds the gateway does not model, and the rest of the
// answer is still usable.
func TestDecodeResponsesAnswer_SkipsUnknownItems(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantText  string
		wantCalls int
	}{
		{
			name:     "an unknown item type is skipped and the text survives",
			body:     `{"status":"completed","output":[{"type":"web_search_call","id":"ws_1"},{"type":"message","content":[{"type":"output_text","text":"found"}]}]}`,
			wantText: "found",
		},
		{
			name:     "a non-object entry in the output array is skipped",
			body:     `{"status":"completed","output":["nope",{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`,
			wantText: "ok",
		},
		{
			name:      "an unknown item type beside a call keeps the call",
			body:      `{"status":"completed","output":[{"type":"web_search_call"},{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{}"}]}`,
			wantCalls: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer, err := decodeResponsesAnswer([]byte(tc.body))
			if err != nil {
				t.Fatalf("decoding the answer: %v", err)
			}
			if text := joinText(answer.Texts); text != tc.wantText {
				t.Fatalf("text = %q, want %q", text, tc.wantText)
			}
			if len(answer.Calls) != tc.wantCalls {
				t.Fatalf("calls = %d, want %d", len(answer.Calls), tc.wantCalls)
			}
		})
	}
}

// TestResponsesTextParts_SkipsEmptyAndNonText pins that only a non-empty
// output_text part contributes: an input_text part belongs to a request, and
// carrying it into an answer would echo the client's own words back.
func TestResponsesTextParts_SkipsEmptyAndNonText(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "an empty text part contributes nothing",
			body: `{"output":[{"type":"message","content":[{"type":"output_text","text":""},{"type":"output_text","text":"b"}]}]}`,
			want: "b",
		},
		{
			name: "an input_text part is not answer text",
			body: `{"output":[{"type":"message","content":[{"type":"input_text","text":"mine"},{"type":"output_text","text":"theirs"}]}]}`,
			want: "theirs",
		},
		{
			name: "a message with no content contributes nothing",
			body: `{"output":[{"type":"message","role":"assistant"}]}`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer, err := decodeResponsesAnswer([]byte(tc.body))
			if err != nil {
				t.Fatalf("decoding the answer: %v", err)
			}
			if text := joinText(answer.Texts); text != tc.want {
				t.Fatalf("text = %q, want %q", text, tc.want)
			}
		})
	}
}
