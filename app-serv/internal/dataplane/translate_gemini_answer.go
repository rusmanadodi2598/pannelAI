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
		ID   string          `json:"id"`
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"functionCall"`
}

// geminiCallRead is one functionCall part as a client needs it: the call named,
// its arguments as the JSON string the OpenAI wire carries, and an id.
type geminiCallRead struct {
	id        string
	name      string
	arguments []byte
}

// call reads one answer part as a tool call, returning false for a part that is
// not one.
//
// The id is the vendor's when the answer carried one. Gemini names none on the
// plain API, and an OpenAI client dispatches and answers by id, so the gateway
// mints one: the call's name and the answer's own creation second make it
// different from the same tool's next turn, and the request direction
// re-uniquifies whatever the client echoes back.
func (p *geminiAnswerPart) call(created int64, index int) (geminiCallRead, bool) {
	if p.FunctionCall == nil || p.FunctionCall.Name == "" {
		return geminiCallRead{}, false
	}
	call := geminiCallRead{name: p.FunctionCall.Name, id: p.FunctionCall.ID}
	// OpenAI clients parse the arguments field as JSON, so a call that arrived
	// with none gets the empty object rather than a field that fails to parse.
	if len(p.FunctionCall.Args) == 0 || string(p.FunctionCall.Args) == "null" {
		call.arguments = []byte("{}")
	} else {
		call.arguments = p.FunctionCall.Args
	}
	if call.id == "" {
		call.id = geminiCallID(call.name, created, index)
	}
	return call, true
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
		if call, ok := part.call(created, len(message.ToolCalls)); ok {
			message.ToolCalls = append(message.ToolCalls, schema.ToolCall{
				ID:       call.id,
				Type:     schema.BlockFunction,
				Function: schema.FunctionCall{Name: call.name, Arguments: string(call.arguments)},
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
	if metadata, ok := geminiUsage(response); ok {
		usage = GeminiUsageToOpenAI(metadata)
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

// geminiAnswerParts reads the answer's parts and finish reason, from inside the
// Cloud Code Assist wrapper when the vendor used one.
func geminiAnswerParts(body object) ([]geminiAnswerPart, string) {
	return readGeminiPartsAndReason(geminiBody(body))
}

// geminiBody returns the object the Gemini members are actually written in: the
// OAuth families nest the same answer under a top-level `response`, while a plain
// generateContent call writes it at the root. Both spellings are one body here
// rather than a rule each reader repeats.
func geminiBody(body object) object {
	if inner, ok := objectField(body, "response"); ok && len(inner) > 0 {
		return inner
	}
	return body
}

// geminiUsageOf returns the billed numbers for a Gemini body, or nil when it
// reported none. The engine's usage reader calls this so the wrapper rule lives in
// one function rather than in each place a Gemini answer is read.
func geminiUsageOf(body object) *schema.Usage {
	metadata, ok := geminiUsage(body)
	if !ok {
		return nil
	}
	parsed := GeminiUsageToOpenAI(metadata)
	return &parsed
}

// geminiUsage returns the accounting block, which travels beside the candidate in
// a wrapped answer and on the payload itself in a flat one. An empty member is not
// numbers, so it is not treated as a report.
func geminiUsage(body object) (json.RawMessage, bool) {
	if metadata, ok := objectField(geminiBody(body), "usageMetadata"); ok && len(metadata) > 0 {
		return mustJSON(metadata), true
	}
	metadata, ok := objectField(body, "usageMetadata")
	if !ok || len(metadata) == 0 {
		return nil, false
	}
	return mustJSON(metadata), true
}

// readGeminiPartsAndReason is the unwrapped reader, split out so the wrapper rule
// lives in geminiBody and not inside the walk.
func readGeminiPartsAndReason(body object) ([]geminiAnswerPart, string) {
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
	feedback, ok := objectField(geminiBody(body), "promptFeedback")
	if !ok {
		return ""
	}
	return stringField(feedback, "blockReason")
}

// geminiCallID mints the id a call the vendor named none carries. It has to hold
// together the call and its result inside one conversation, and be different from
// the same tool's call on the next turn, because Gemini validates functionCall ids
// across the whole history.
func geminiCallID(name string, created int64, index int) string {
	return fmt.Sprintf("call_%s_%d_%d", name, created, index)
}
