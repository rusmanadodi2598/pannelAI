// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_forced_chat.go
// @for       Accumulating an OpenAI chat stream into the single completion it
//
//	stands for.
//
// @uses      internal/schema, sort, strconv.
// @reason    A provider that forces streaming still has to answer a client that
//
//	asked for one JSON body, and a chat stream reports that answer as
//	deltas rather than as one object. The accumulation rules (fragments
//	join, tool calls attach by index, the last finish reason wins) live
//	here so the fold itself stays about reading events.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-21
package dataplane

import (
	"sort"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// foldedChat accumulates the frames of one chat stream.
type foldedChat struct {
	id      string
	model   string
	created int64
	content string
	reason  string
	finish  string
	// calls holds the tool calls by the index the stream reported, so an
	// argument fragment lands on the call it belongs to.
	calls map[int]*schema.ToolCall
	usage *schema.Usage
	// stop holds the sequences the caller asked the answer to end at, which an
	// upstream on this wire may ignore; the fold is where the cut still applies
	// because the whole text is assembled before anyone reads it.
	stop []string
	// seen counts the chunks that carried a choice, so a stream that produced
	// nothing is reported rather than answered with an empty completion.
	seen int
}

// absorb folds one chunk into the accumulated answer.
func (f *foldedChat) absorb(chunk object) {
	if id := stringField(chunk, "id"); id != "" {
		f.id = id
	}
	// The resolved model arrives seeded, and an upstream alias only fills a name
	// that was never resolved: Qoder labels every model it serves `auto`, and a
	// caller that reads it back gets a model it cannot ask for again.
	if model := stringField(chunk, "model"); model != "" && f.model == "" {
		f.model = model
	}
	if created := intField(chunk, "created"); created > 0 {
		f.created = int64(created)
	}
	if usage, ok := objectField(chunk, "usage"); ok {
		f.usage = openAIUsageFromObject(usage)
	}

	choices, ok := arrayField(chunk, "choices")
	if !ok || len(choices) == 0 {
		// A usage-only frame carries no choice, which is the shape
		// stream_options.include_usage asks for.
		return
	}
	choice, ok := decodeObject(choices[0])
	if !ok {
		return
	}
	f.seen++
	if reason := stringField(choice, "finish_reason"); reason != "" {
		f.finish = reason
	}
	delta, ok := objectField(choice, "delta")
	if !ok {
		return
	}
	f.content += stringField(delta, "content")
	f.reason += reasoningText(delta)
	f.absorbCalls(delta)
}

// absorbCalls attaches one frame's tool-call fragments to the calls they belong
// to. A fragment may open a call, name it, or extend its arguments, and the three
// can arrive in any order across frames.
func (f *foldedChat) absorbCalls(delta object) {
	entries, ok := arrayField(delta, "tool_calls")
	if !ok {
		return
	}
	for _, raw := range entries {
		fragment, ok := decodeObject(raw)
		if !ok {
			continue
		}
		index := intField(fragment, "index")
		call, present := f.calls[index]
		if !present {
			call = &schema.ToolCall{Type: schema.BlockFunction}
			f.calls[index] = call
		}
		if id := stringField(fragment, "id"); id != "" {
			call.ID = id
		}
		if callType := stringField(fragment, "type"); callType != "" {
			call.Type = callType
		}
		function, ok := objectField(fragment, "function")
		if !ok {
			continue
		}
		call.Function.Name += stringField(function, "name")
		call.Function.Arguments += stringField(function, "arguments")
	}
}

// response renders the accumulated frames as the non-streamed completion they
// stand for.
func (f *foldedChat) response() schema.ChatCompletionResponse {
	message := schema.ChatMessage{
		Role:      RoleAssistant,
		Content:   schema.MessageContent{Text: f.content},
		Reasoning: f.reason,
	}
	// The caller's `stop` is honoured here even when the upstream answered past
	// it, and a cut is a stop: an upstream that ran on to its own ceiling after
	// the marker would otherwise report `length` for text it never showed.
	trimmed, _, stopped := cutAtStop(message.Content.Text, f.stop)
	message.Content.Text = trimmed
	if len(f.calls) > 0 {
		message.ToolCalls = f.callsByIndex()
		if message.Content.Text == "" {
			// An answer that is only tool calls reports null content, which is
			// what a client dispatches on rather than an empty string.
			message.Content = schema.MessageContent{}
		}
	}
	// The thinking a folded stream accumulated stays on the answer. The reference
	// strips it once content is non-empty (sseToJsonHandler.js), but this gateway
	// bills it either way: a caller charged for 120 reasoning tokens that it can
	// neither read nor cite is charged for something that never reached it, and a
	// streaming client of the same model sees the same text frame by frame. One
	// body and one stream now say the same thing.
	finish := f.finish
	if stopped {
		finish = FinishStop
	}
	if finish == "" {
		finish = FinishStop
	}
	return schema.ChatCompletionResponse{
		ID:      f.id,
		Object:  "chat.completion",
		Created: f.created,
		Model:   f.model,
		Choices: []schema.ChatChoice{{Index: 0, Message: message, FinishReason: finish}},
		Usage:   f.usage,
	}
}

// callsByIndex returns the tool calls in the order the stream indexed them, which
// is the order a client dispatches them in.
func (f *foldedChat) callsByIndex() []schema.ToolCall {
	indices := make([]int, 0, len(f.calls))
	for index := range f.calls {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	calls := make([]schema.ToolCall, 0, len(indices))
	for _, index := range indices {
		calls = append(calls, *f.calls[index])
	}
	return calls
}
