// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/token_saver_translate.go
// @for       The Headroom pivot for Claude and Responses upstream bodies.
// @uses      internal/schema, internal/tokensaver, bytes, encoding/json, errors.
// @reason    Headroom accepts OpenAI chat messages only, while the data plane may
//
// dispatch an already translated Claude or Responses body. This adapter
// reuses the existing request translators for the pivot, then replaces
// only the compressed wire members in the original envelope so fields the
// gateway does not model still reach the provider unchanged.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"errors"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/tokensaver"
)

// TokenSaverTranslator is the full dataplane adapter for Headroom. It is passed
// to tokensaver.Applier by the composition root, keeping the saver package free
// of the dataplane's translator graph.
type TokenSaverTranslator struct{}

// Prepare pivots one upstream body into Headroom's OpenAI messages array. An
// OpenAI body is already in the proxy shape; Responses bodies with tool or
// reasoning items are deliberately refused because their ordering cannot be
// safely represented by a chat message array.
func (TokenSaverTranslator) Prepare(body []byte, wire, model string) (tokensaver.HeadroomRequest, bool, error) {
	switch wire {
	case TargetOpenAI:
		return prepareOpenAIHeadroom(body, model)
	case TargetClaude:
		return prepareClaudeHeadroom(body, model)
	case TargetResponses:
		return prepareResponsesHeadroom(body, model)
	default:
		return tokensaver.HeadroomRequest{}, false, nil
	}
}

// Restore puts Headroom's compressed messages back into the original upstream
// wire, preserving all unrelated top-level members.
func (TokenSaverTranslator) Restore(body []byte, wire, model string, messages json.RawMessage) ([]byte, error) {
	switch wire {
	case TargetOpenAI:
		return restoreOpenAIHeadroom(body, messages)
	case TargetClaude:
		return restoreClaudeHeadroom(body, model, messages)
	case TargetResponses:
		return restoreResponsesHeadroom(body, model, messages)
	default:
		return body, nil
	}
}

// prepareOpenAIHeadroom validates and carries the native messages array.
func prepareOpenAIHeadroom(body []byte, model string) (tokensaver.HeadroomRequest, bool, error) {
	envelope, ok := decodeTokenSaverObject(body)
	if !ok {
		return tokensaver.HeadroomRequest{}, false, nil
	}
	messages, ok := envelope["messages"]
	if !ok || !isTokenSaverArray(messages) {
		return tokensaver.HeadroomRequest{}, false, nil
	}
	return tokensaver.HeadroomRequest{Messages: messages, Model: model}, true, nil
}

// prepareClaudeHeadroom decodes the Claude wire and uses the existing Claude to
// OpenAI translator as the proxy pivot.
func prepareClaudeHeadroom(body []byte, model string) (tokensaver.HeadroomRequest, bool, error) {
	var request schema.MessagesRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return tokensaver.HeadroomRequest{}, false, err
	}
	pivot := ClaudeToOpenAI(request, model, request.Stream)
	encoded, err := json.Marshal(pivot)
	if err != nil {
		return tokensaver.HeadroomRequest{}, false, err
	}
	return prepareOpenAIHeadroom(encoded, model)
}

// prepareResponsesHeadroom only accepts an input list made solely of message
// items. The proxy cannot preserve Responses tool calls or reasoning items.
func prepareResponsesHeadroom(body []byte, model string) (tokensaver.HeadroomRequest, bool, error) {
	envelope, ok := decodeTokenSaverObject(body)
	if !ok || !responsesInputIsSafe(envelope["input"]) {
		return tokensaver.HeadroomRequest{}, false, nil
	}
	var request schema.ResponsesRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return tokensaver.HeadroomRequest{}, false, err
	}
	pivot := ResponsesToOpenAI(request, model, request.Stream)
	encoded, err := json.Marshal(pivot)
	if err != nil {
		return tokensaver.HeadroomRequest{}, false, err
	}
	return prepareOpenAIHeadroom(encoded, model)
}

// restoreOpenAIHeadroom replaces just messages in a native OpenAI body.
func restoreOpenAIHeadroom(body []byte, messages json.RawMessage) ([]byte, error) {
	envelope, ok := decodeTokenSaverObject(body)
	if !ok || !isTokenSaverArray(messages) {
		return body, errors.New("token saver: invalid OpenAI Headroom result")
	}
	envelope["messages"] = messages
	return marshalTokenSaverObject(envelope)
}

// restoreClaudeHeadroom maps compressed chat messages back into Claude message
// blocks and copies only messages/system into the original envelope.
func restoreClaudeHeadroom(body []byte, model string, messages json.RawMessage) ([]byte, error) {
	var original schema.MessagesRequest
	if err := json.Unmarshal(body, &original); err != nil {
		return body, err
	}
	var compressed []schema.ChatMessage
	if err := json.Unmarshal(messages, &compressed); err != nil {
		return body, err
	}
	pivot := ClaudeToOpenAI(original, model, original.Stream)
	pivot.Messages = compressed
	converted := OpenAIToClaude(pivot, model, original.Stream)
	convertedRaw, err := json.Marshal(converted)
	if err != nil {
		return body, err
	}
	return replaceTokenSaverMembers(body, convertedRaw, "messages", "system")
}

// restoreResponsesHeadroom maps compressed chat messages back into Responses
// message items and copies only input/instructions into the original envelope.
func restoreResponsesHeadroom(body []byte, model string, messages json.RawMessage) ([]byte, error) {
	var original schema.ResponsesRequest
	if err := json.Unmarshal(body, &original); err != nil {
		return body, err
	}
	var compressed []schema.ChatMessage
	if err := json.Unmarshal(messages, &compressed); err != nil {
		return body, err
	}
	pivot := ResponsesToOpenAI(original, model, original.Stream)
	pivot.Messages = compressed
	converted := OpenAIToResponses(pivot, model, original.Stream)
	convertedRaw, err := json.Marshal(converted)
	if err != nil {
		return body, err
	}
	return replaceTokenSaverMembers(body, convertedRaw, "input", "instructions")
}

// replaceTokenSaverMembers copies selected members from translated into body
// while leaving every other member's raw JSON untouched.
func replaceTokenSaverMembers(body, translated []byte, members ...string) ([]byte, error) {
	original, ok := decodeTokenSaverObject(body)
	if !ok {
		return body, errors.New("token saver: original body is not an object")
	}
	converted, ok := decodeTokenSaverObject(translated)
	if !ok {
		return body, errors.New("token saver: translated body is not an object")
	}
	for _, member := range members {
		if value, exists := converted[member]; exists {
			original[member] = value
		}
	}
	return marshalTokenSaverObject(original)
}
