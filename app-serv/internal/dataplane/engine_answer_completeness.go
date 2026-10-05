// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_answer_completeness.go
// @for       Reporting whether a non-streamed upstream answer carries nothing a
//
//	client can show and stopped because of its own output ceiling.
//
// @uses      internal/schema, encoding/json, strings.
// @reason    An upstream that spends the whole output ceiling on reasoning answers
//
//	200 with an empty body, a success as far as the transport is
//	concerned, and what a combo would otherwise hand the client. The
//	signal is read through the translators every upstream format already
//	has, the way readUsage does, so the rule exists once instead of as one
//	hand-written wire reader per format.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package dataplane

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// emptyTruncatedAnswer reports whether a non-streamed upstream answer is one the
// client cannot read anything out of, cut short by its output ceiling.
//
// It is deliberately narrower than "this answer looks bad": only a ceiling stop
// counts, because that is the failure a larger ceiling would not have. An answer
// that stopped for another reason, or that carries anything showable, is the
// upstream's answer and is served as written.
func emptyTruncatedAnswer(raw []byte, resolution Resolution) bool {
	completion, ok := readCompletion(raw, resolution)
	if !ok || len(completion.Choices) == 0 {
		return false
	}
	stopped := false
	for _, choice := range completion.Choices {
		if answerShowable(choice.Message) {
			return false
		}
		if choice.FinishReason == FinishLength {
			stopped = true
		}
	}
	return stopped
}

// answerShowable reports whether a message holds anything a client can render:
// text in either content shape, a tool call to dispatch, or reasoning.
//
// Reasoning counts as an answer because the fold keeps it precisely when the
// content is empty (engine_forced_chat.go:138-147); a thinking-only reply is thin,
// but it is what the model chose to say.
func answerShowable(message schema.ChatMessage) bool {
	if strings.TrimSpace(message.Content.TextContent()) != "" {
		return true
	}
	return message.Reasoning != "" || len(message.ToolCalls) > 0
}

// readCompletion reads a non-streamed upstream answer into the OpenAI completion
// shape, whichever wire the upstream wrote it on. The two translators it delegates
// to already map every vendor's stop reason onto finish_reason, so a caller reads
// one vocabulary rather than three.
func readCompletion(raw []byte, resolution Resolution) (schema.ChatCompletionResponse, bool) {
	switch resolution.Target {
	case TargetOpenAI:
		var completion schema.ChatCompletionResponse
		if err := json.Unmarshal(raw, &completion); err != nil {
			return schema.ChatCompletionResponse{}, false
		}
		return completion, true
	case TargetClaude:
		completion, err := ClaudeToOpenAIResponse(raw, resolution.ModelID, 0)
		return completion, err == nil
	case TargetResponses:
		completion, err := ResponsesToOpenAIResponse(raw, resolution.ModelID, 0)
		return completion, err == nil
	default:
		// No translator reads a wire the resolver can name here, so nothing is
		// known about its completeness and the answer is served unchanged.
		return schema.ChatCompletionResponse{}, false
	}
}
