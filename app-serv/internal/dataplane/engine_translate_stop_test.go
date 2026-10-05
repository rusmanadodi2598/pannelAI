// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_translate_stop_test.go
// @for       The caller's `stop` reaching the upstream in one shape and cutting the
//
//	folded answer in the other.
//
// @uses      encoding/json, strings, testing, internal/schema.
// @reason    The two halves are the two directions of the same promise, and both
//
//	were measured wrong live: a string-form `stop` drew a vendor refusal
//	that the array form did not, and an array-form `stop` changed nothing
//	at all because the upstream ignored it. A folded answer is where a
//	non-streaming caller's text is assembled, so that is where the cut
//	has to land.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-30
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeStopMember_SendsTheArrayFormTheVendorAccepts(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		want  string
		keeps bool
	}{
		{name: "a single string becomes a one-element array", body: `{"model":"m","stop":"green"}`, want: `"stop":["green"]`},
		{name: "an array is left as it arrived", body: `{"model":"m","stop":["green","red"]}`, want: `"stop":["green","red"]`, keeps: true},
		{name: "no member is added", body: `{"model":"m"}`, want: `{"model":"m"}`, keeps: true},
		{name: "a null member is left alone", body: `{"model":"m","stop":null}`, want: `"stop":null`, keeps: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeStopMember([]byte(tc.body))
			if err != nil {
				t.Fatalf("normalizeStopMember() error = %v", err)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Fatalf("body = %s, want it to carry %s", got, tc.want)
			}
			if tc.keeps && string(got) != tc.body {
				t.Fatalf("body = %s, want the bytes untouched", got)
			}
		})
	}
}

func TestNormalizeStopMember_KeepsEveryOtherMember(t *testing.T) {
	body := `{"model":"m","stop":"green","temperature":0.5,"tools":[{"type":"function"}]}`
	got, err := normalizeStopMember([]byte(body))
	if err != nil {
		t.Fatalf("normalizeStopMember() error = %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("the rewritten body does not decode: %v", err)
	}
	for _, member := range []string{"model", "temperature", "tools"} {
		if _, present := decoded[member]; !present {
			t.Fatalf("%s was lost while rewriting stop: %s", member, got)
		}
	}
	if len(decoded) != 4 {
		t.Fatalf("the rewrite changed the member count: %s", got)
	}
}

func TestFoldChatEvents_CutsTheAnswerAtTheStopSequence(t *testing.T) {
	events := [][]byte{
		[]byte(`{"choices":[{"index":0,"delta":{"content":"keep "}}]}`),
		[]byte(`{"choices":[{"index":0,"delta":{"content":"CUT drop this"}}]}`),
		[]byte(`{"choices":[{"index":0,"delta":{"content":" and more"},"finish_reason":"length"}]}`),
	}
	body, _, err := foldChatEvents(events, "codebuddy-intl/deepseek-v4.1-flash", []string{"CUT"})
	if err != nil {
		t.Fatalf("foldChatEvents() error = %v", err)
	}
	var answer struct {
		Model   string `json:"model"`
		Choices []struct {
			Message      struct{ Content string } `json:"message"`
			FinishReason string                   `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("the folded answer does not decode: %v", err)
	}
	if answer.Choices[0].Message.Content != "keep " {
		t.Fatalf("content = %q, want the text before the marker only", answer.Choices[0].Message.Content)
	}
	// The marker is where the caller ended the answer, so the vendor's own
	// ceiling is not what the client is told.
	if answer.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason = %q, want stop", answer.Choices[0].FinishReason)
	}
	if answer.Model != "codebuddy-intl/deepseek-v4.1-flash" {
		t.Fatalf("model = %q, want the name the caller addressed", answer.Model)
	}
}

func TestFoldChatEvents_LeavesTheAnswerWhenNoMarkerArrives(t *testing.T) {
	events := [][]byte{
		[]byte(`{"choices":[{"index":0,"delta":{"content":"all of it CUT"}}]}`),
	}
	body, _, err := foldChatEvents(events, "m", []string{"NOPE"})
	if err != nil {
		t.Fatalf("foldChatEvents() error = %v", err)
	}
	if !strings.Contains(string(body), `"content":"all of it CUT"`) {
		t.Fatalf("a near-miss marker cut a real answer: %s", body)
	}
}
