// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_body.go
// @for       The agent payload Qoder's chat endpoint answers, built from the request the gateway already translated.
// @uses      crypto/sha256, encoding/hex, encoding/json, fmt, strings, time.
// @reason    Qoder does not read an OpenAI body. It reads an agent request whose routing node is named, whose session and record ids the vendor dedupes on, and whose model is chosen by the configuration the vendor published for that account, a wrong or absent `model_config` is answered with a different model, silently (measured, draft 036 §5.1: a plain OpenAI body is refused with "None flow nodes found").
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// qoderDefaultMaxTokens is the ceiling the reference sends when the vendor's model
// configuration states none.
const qoderDefaultMaxTokens = 32_768

// TransformRequest rewrites the translated chat body into the agent payload. It is
// where the vendor's own model configuration is fetched, because that read is signed
// as the account and the body cannot be finished without it.
func (c *Qoder) TransformRequest(req *Request) error {
	var incoming qoderOpenAIRequest
	if err := json.Unmarshal(req.Body, &incoming); err != nil {
		return fmt.Errorf("provider %s: the request body could not be read: %w", c.entry.ID, err)
	}
	modelKey := qoderModelKey(req, incoming.Model)

	messages, systemText := normalizeQoderMessages(incoming.Messages)
	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}
	config, err := c.modelConfig(ctx, req.Credential, modelKey)
	if err != nil {
		return err
	}
	maxTokens := qoderMaxTokens(config, incoming)
	recordID := qoderChatRecordID(modelKey, messages, incoming.Tools, maxTokens)
	tools := parseQoderTools(incoming.Tools)
	lastUser := lastQoderUserText(messages)

	built := qoderAgentRequest{
		RequestID:    newQoderID(),
		RequestSetID: recordID, ChatRecordID: recordID,
		SessionID: qoderStableID("qoder-session", req.Credential.ProjectID(), modelKey),
		Stream:    true,
		ChatTask:  "FREE_INPUT", IsReply: true, Source: 1,
		Version: "3", SessionType: "qodercli",
		AgentID: "agent_common", TaskID: "common",
		ImageURLs:  nil,
		System:     systemText,
		Messages:   messages,
		Tools:      tools,
		Parameters: qoderParameters{MaxTokens: maxTokens},
		ChatContext: qoderChatContext{Extra: qoderChatContextExtra{
			Context: []string{}, ModelConfig: qoderModelSelection{Key: modelKey, IsReasoning: qoderIsReasoning(config)},
			OriginalContent: lastUser,
		}, Features: []string{}, Text: lastUser},
		ModelConfig: config,
		Business: qoderBusiness{Product: "cli", Version: qoderIDEVersion, Type: "agent",
			Stage: "start", ID: newQoderID(), Name: truncateQoder(lastUser, 30),
			BeginAt: time.Now().UnixMilli()},
	}

	raw, err := json.Marshal(built)
	if err != nil {
		return fmt.Errorf("provider %s: the agent request could not be encoded: %w", c.entry.ID, err)
	}
	req.Body = raw
	return nil
}

// ForcesStream implements provider.StreamForcer: this endpoint is an SSE service, and
// the payload above is built for a stream, so a client that asked for one JSON body
// is served from the stream rather than left with an answer the vendor will not give.
func (c *Qoder) ForcesStream() bool { return true }

// qoderChatRecordID and qoderStableID are content-addressed, so a retried call keeps
// the identity the vendor dedupes on and a changed prompt does not. The truncated
// digest is the shape the vendor's client sends.
func qoderChatRecordID(model string, messages []qoderMessage, tools json.RawMessage, maxTokens int) string {
	hash := sha256.New()
	hash.Write([]byte("qoder-record\x00" + model))
	for _, message := range messages {
		hash.Write([]byte("\x00" + message.Role))
		hash.Write(qoderRawForHash(message.Content))
	}
	hash.Write([]byte("\x00"))
	hash.Write(qoderRawForHash(tools))
	hash.Write([]byte("\x00mt=" + strconv.Itoa(maxTokens)))
	return hex.EncodeToString(hash.Sum(nil))[:16]
}

func qoderStableID(prefix string, parts ...string) string {
	hash := sha256.New()
	hash.Write([]byte(prefix))
	for _, part := range parts {
		hash.Write([]byte("\x00" + part))
	}
	return hex.EncodeToString(hash.Sum(nil))[:16]
}

// qoderRawForHash normalises the whitespace json.RawMessage may carry, so the same
// prompt encoded two ways keeps one identity.
func qoderRawForHash(raw json.RawMessage) []byte {
	var compact bytes.Buffer
	if json.Compact(&compact, raw) != nil {
		return raw
	}
	return compact.Bytes()
}

func truncateQoder(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
