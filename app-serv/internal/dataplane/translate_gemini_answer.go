// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_gemini_answer.go
// @for       The inbound Gemini direction: one answer or one streamed frame read as the OpenAI shape the client asked for.
// @uses      encoding/json, internal/schema.
// @reason    Gemini writes an answer where OpenAI does not look: candidates rather than choices, parts rather than a message, usageMetadata rather than usage, and a blocked prompt that arrives as a 200 with no answer at all. Reading those as OpenAI would produce an empty completion and a zero-token bill, which is why each difference has a named rule here instead of a parser pointed at a shape it was not written for.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package dataplane

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// concatGeminiText joins answer fragments without a separator: parts inside one
// candidate are consecutive spans of the same text, so the newline join the block
// shape uses would insert characters the model never wrote.
func concatGeminiText(values []string) string {
	return strings.Join(values, "")
}

// geminiAnswerPart is one part of a Gemini answer. The read shape is its own
// rather than the request type's because a thought carries a boolean the request
// direction never sends, and adding it there would put `"thought":false` on every
// outbound payload.
type geminiAnswerPart struct {
	Text    string `json:"text"`
	Thought bool   `json:"thought"`

	FunctionCall *struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"functionCall"`
}

func (p *geminiAnswerPart) functionCall() (string, []byte, bool) {
	if p.FunctionCall == nil || p.FunctionCall.Name == "" {
		return "", nil, false
	}
	arguments := p.FunctionCall.Args
	// OpenAI clients parse the arguments field as JSON, so a call that arrived
	// with none gets the empty object rather than a field that fails to parse.
	if len(arguments) == 0 || string(arguments) == "null" {
		arguments = []byte("{}")
	}
	return p.FunctionCall.Name, arguments, true
}

// GeminiToOpenAIResponse translates one non-streamed Gemini answer into the
// OpenAI completion a client reads.
//
// Only the first candidate becomes the answer: a completion carries one message
// per choice, and a caller asking for several candidates sees the rest dropped
// rather than counted. A part marked thinking is reasoning, not content, and a
// function call makes the finish reason a call unless the upstream said the
// answer was cut off, because a client reading "stop" never sends the result back.
func GeminiToOpenAIResponse(raw []byte, model string, created int64) (schema.ChatCompletionResponse, error) {
	response, ok := decodeObject(raw)
	if !ok {
		return schema.ChatCompletionResponse{}, dataPlaneError(CodeUpstreamError, "the upstream answered with an unreadable body")
	}

	message := schema.ChatMessage{Role: RoleAssistant}
	texts := make([]string, 0, 2)
	parts, reason := geminiAnswerParts(response)
	for _, part := range parts {
		switch {
		case part.Thought:
			if part.Text != "" {
				message.Reasoning += part.Text
			}
		case part.Text != "":
			texts = append(texts, part.Text)
		}
		if name, arguments, ok := part.functionCall(); ok {
			message.ToolCalls = append(message.ToolCalls, schema.ToolCall{
				ID:       geminiCallID(len(message.ToolCalls)),
				Type:     schema.BlockFunction,
				Function: schema.FunctionCall{Name: name, Arguments: string(arguments)},
			})
		}
	}

	finish := openAIFinishReason(reason, formatGemini)
	if len(message.ToolCalls) > 0 && finish == FinishStop {
		finish = FinishToolCalls
	}
	// A prompt the vendor refused never reaches a candidate, so the block reason
	// is the only answer there is, and an empty completion is a different claim.
	if reason == "" && len(parts) == 0 && geminiBlockReason(response) != "" {
		finish = FinishContentFilter
	}
	message.Content = schema.MessageContent{Text: concatGeminiText(texts)}

	usage := schema.Usage{}
	if metadata, ok := objectField(response, "usageMetadata"); ok && len(metadata) > 0 {
		usage = GeminiUsageToOpenAI(mustJSON(metadata))
	}
	return schema.ChatCompletionResponse{
		ID:      responseID("", "chatcmpl-pannelai"),
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []schema.ChatChoice{{Index: 0, Message: message, FinishReason: finish}},
		Usage:   &usage,
	}, nil
}

// geminiAnswerAsOpenAI renders a Gemini answer as an OpenAI completion body, so a
// client on the Anthropic or Responses wire reads the answer through the reader it
// was written for rather than a second copy of the candidate rules.
func geminiAnswerAsOpenAI(raw []byte, resolution Resolution, created int64) ([]byte, error) {
	answer, err := GeminiToOpenAIResponse(raw, answerModel(resolution, resolution.ModelID), created)
	if err != nil {
		return nil, err
	}
	return mustJSON(answer), nil
}

// geminiAnswerParts reads the first candidate's parts and its finish reason.
func geminiAnswerParts(body object) ([]geminiAnswerPart, string) {
	candidates, ok := arrayField(body, "candidates")
	if !ok || len(candidates) == 0 {
		return nil, ""
	}
	candidate, ok := decodeObject(candidates[0])
	if !ok {
		return nil, ""
	}
	reason := stringField(candidate, "finishReason")

	content, ok := objectField(candidate, "content")
	if !ok {
		return nil, reason
	}
	raw, ok := arrayField(content, "parts")
	if !ok {
		return nil, reason
	}
	parts := make([]geminiAnswerPart, 0, len(raw))
	for _, item := range raw {
		part := geminiAnswerPart{}
		if err := json.Unmarshal(item, &part); err == nil {
			parts = append(parts, part)
		}
	}
	return parts, reason
}

// geminiBlockReason names the reason a prompt never reached a candidate.
func geminiBlockReason(body object) string {
	feedback, ok := objectField(body, "promptFeedback")
	if !ok {
		return ""
	}
	return stringField(feedback, "blockReason")
}

// geminiCallID synthesizes the call id an OpenAI client needs: Gemini sends none,
// and the request direction pairs a result back to its call by function name, so
// an id only has to be distinct within one answer.
func geminiCallID(index int) string {
	return fmt.Sprintf("call_%d", index)
}
