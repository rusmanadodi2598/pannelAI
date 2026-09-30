// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/stop_claude_answer_test.go
// @for       The caller's `stop_sequences` on a one-body Anthropic answer.
// @uses      strings, testing, internal/schema.
// @reason    The single-body shape is where a caller reads the whole answer at once,
//
//	so the cut has to name the marker it honoured and leave a `tool_use`
//	block alone. Measured live on 2026-09-30 through /api/v1/messages: the
//	answer came back `A STOPHERE B` with `stop_reason: end_turn`.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-30
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

func answersBlock(text string) string {
	return `[{"type":"` + schema.BlockText + `","text":"` + text + `"}]`
}

func TestCutClaudeAnswer_TrimsTheOneBodyAnswerAndNamesTheMarker(t *testing.T) {
	body := `{"id":"msg_1","type":"message","role":"assistant","model":"m","content":` +
		answersBlock("A STOPHERE B") + `,"stop_reason":"end_turn","stop_sequence":null,` +
		`"usage":{"input_tokens":3,"output_tokens":4}}`
	cut := string(cutClaudeAnswer([]byte(body), []string{"STOPHERE"}))
	if strings.Contains(cut, "STOPHERE B") {
		t.Fatalf("text past the marker survived: %s", cut)
	}
	if !strings.Contains(cut, `"text":"A "`) {
		t.Fatalf("the text before the marker was lost: %s", cut)
	}
	if !strings.Contains(cut, `"stop_reason":"stop_sequence"`) || !strings.Contains(cut, `"stop_sequence":"STOPHERE"`) {
		t.Fatalf("the cut was not reported as a stop sequence: %s", cut)
	}
	// Members the schema does not model travel untouched through the rewrite.
	if !strings.Contains(cut, `"usage":{"input_tokens":3,"output_tokens":4}`) {
		t.Fatalf("usage was dropped by the rewrite: %s", cut)
	}
}

func TestCutClaudeAnswer_KeepsAToolUseBlockAfterTheMarker(t *testing.T) {
	body := `{"content":[{"type":"` + schema.BlockText + `","text":"calling "},` +
		`{"type":"` + schema.BlockToolUse + `","id":"t1","name":"read","input":{}},` +
		`{"type":"` + schema.BlockText + `","text":"STOPHERE trailing prose"}],` +
		`"stop_reason":"tool_use","stop_sequence":null}`
	cut := string(cutClaudeAnswer([]byte(body), []string{"STOPHERE"}))
	if !strings.Contains(cut, `"id":"t1"`) {
		t.Fatalf("a tool call was deleted as collateral of a text cut: %s", cut)
	}
	if strings.Contains(cut, "trailing prose") {
		t.Fatalf("text after the marker survived: %s", cut)
	}
}

func TestCutClaudeAnswer_FindsAMarkerSpanningTwoTextBlocks(t *testing.T) {
	body := `{"content":[{"type":"` + schema.BlockText + `","text":"one STOP"},` +
		`{"type":"` + schema.BlockText + `","text":"HERE two"}],"stop_reason":"end_turn","stop_sequence":null}`
	cut := string(cutClaudeAnswer([]byte(body), []string{"STOPHERE"}))
	if !strings.Contains(cut, `"stop_sequence":"STOPHERE"`) {
		t.Fatalf("a marker split across blocks was not caught: %s", cut)
	}
	if strings.Contains(cut, "HERE two") {
		t.Fatalf("the block after the split marker survived: %s", cut)
	}
}

func TestFoldStopSequences_DefersTheCutToTheAnthropicShape(t *testing.T) {
	// Cutting the folded OpenAI body first would remove the marker before the
	// Anthropic answer exists, leaving the text right and the reason wrong.
	chat := schema.ChatRequest{Stop: json.RawMessage(`["STOPHERE"]`)}
	messages := schema.MessagesRequest{StopSequences: []string{"STOPHERE"}}

	openAIClient := Request{ClientFormat: schema.FormatOpenAI, Chat: &chat}
	if got := foldStopSequences(openAIClient); len(got) != 1 || got[0] != "STOPHERE" {
		t.Fatalf("fold stop = %v, want the OpenAI client cut in the fold", got)
	}
	claudeClient := Request{ClientFormat: schema.FormatAnthropic, Messages: &messages}
	if got := foldStopSequences(claudeClient); got != nil {
		t.Fatalf("fold stop = %v, want nil: the Anthropic answer is cut after translation", got)
	}
	// The client's own sequences still travel, so the later cut has them.
	if got := claudeClient.stopSequences(); len(got) != 1 || got[0] != "STOPHERE" {
		t.Fatalf("stopSequences() = %v, want the caller's marker", got)
	}
}

func TestCutClaudeAnswer_LeavesTheBodyWhenNothingMatches(t *testing.T) {
	bodies := []string{
		`{"content":[{"type":"text","text":"plain answer"}],"stop_reason":"end_turn","stop_sequence":null}`,
		`{"content":[],"stop_reason":"end_turn"}`,
		`{"content":"not an array at all"}`,
		`not json`,
	}
	for _, body := range bodies {
		if got := string(cutClaudeAnswer([]byte(body), []string{"STOPHERE"})); got != body {
			t.Fatalf("body = %s, want it returned untouched", got)
		}
	}
	// A caller that named no sequences has nothing cut, marker or not.
	withMarker := `{"content":[{"type":"text","text":"A STOPHERE B"}],"stop_reason":"end_turn"}`
	if got := string(cutClaudeAnswer([]byte(withMarker), nil)); got != withMarker {
		t.Fatalf("body = %s, want it untouched when no stop sequence was named", got)
	}
}
