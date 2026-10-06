// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_claude_billing_test.go
// @for       The Claude-to-OpenAI system mapping: the billing header a Claude Code client injects is stripped before the prompt is forwarded.
// @uses      strings, testing, internal/schema.
// @reason    The reference removes the header per system block (claude-to-openai.js stripAnthropicBillingHeader) because it is client-side accounting, not conversation; forwarding it double-reports the session on an Anthropic upstream.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestClaudeToOpenAIStripsTheBillingHeader pins that the injected accounting
// line never reaches the upstream while the real system prompt is kept.
func TestClaudeToOpenAIStripsTheBillingHeader(t *testing.T) {
	request := schema.MessagesRequest{
		Model: "claude-sonnet",
		Messages: []schema.Message{{
			Role:    schema.RoleUser,
			Content: schema.MessageBlocks{{Type: schema.BlockText, Text: "ping"}},
		}},
		System: schema.TextBlocks{
			{Type: schema.BlockText, Text: "x-anthropic-billing-header: session-abc123"},
			{Type: schema.BlockText, Text: "You are a helpful assistant."},
		},
	}

	translated := ClaudeToOpenAI(request, "gpt-x", false)
	if len(translated.Messages) == 0 || translated.Messages[0].Role != schema.RoleSystem {
		t.Fatalf("translated messages = %+v, want a leading system message", translated.Messages)
	}
	text := translated.Messages[0].Content.Text
	if strings.Contains(text, "x-anthropic-billing-header") {
		t.Fatalf("system text = %q, want the billing header stripped", text)
	}
	if !strings.Contains(text, "You are a helpful assistant.") {
		t.Fatalf("system text = %q, want the real system prompt kept", text)
	}
}
