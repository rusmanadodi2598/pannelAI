// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/ping.go
// @for       The one-token probe the combo test route runs against one model
//
//	reference (SPEC-API-001 §7.7).
//
// @uses      internal/schema, context, encoding/json.
// @reason    A probe is an ordinary data plane call — resolve, select, translate,
//
//	call — so it goes through Relay rather than a second HTTP path that
//	would drift from the pipeline. What this file owns is only the request
//	it invents and the ceiling that keeps it cheap.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// PingPrompt is the probe's question. It is one word because the answer's
// content is not read: the probe reports whether the pipeline completed.
const PingPrompt = "ping"

// PingMaxTokens bounds the probe's answer to a single token, so testing a combo
// spends the least an upstream will bill for.
const PingMaxTokens = 1

// Ping runs a one-token chat against one model reference and reports what the
// data plane answered, which is what the combo test route reports per member.
//
// It calls Relay directly rather than going through the chat service, so no
// usage row is written: the probe has no gateway key and no request id to record
// one against, and its spend is an operator diagnostic — the same way a fusion
// panel's calls stay outside the request's usage row.
func (e *Engine) Ping(ctx context.Context, ref string) (Outcome, error) {
	request, err := pingRequest(ref)
	if err != nil {
		return Outcome{}, err
	}
	return e.Relay(ctx, request, nil)
}

// pingRequest is the probe's request: an OpenAI chat with one user turn and a
// one-token ceiling, carrying the raw body the way a client's request does, so
// a same-format target forwards it and a cross-format one translates it.
func pingRequest(ref string) (Request, error) {
	maxTokens := PingMaxTokens
	chat := &schema.ChatRequest{
		Model: ref,
		Messages: []schema.ChatMessage{{
			Role: RoleUser, Content: schema.MessageContent{Text: PingPrompt},
		}},
		MaxTokens: &maxTokens,
	}
	raw, err := json.Marshal(chat)
	if err != nil {
		return Request{}, internalError("the ping request could not be built", err)
	}
	return Request{
		Route:        RouteChatCompletions,
		ClientFormat: schema.FormatOpenAI,
		Model:        ref,
		Chat:         chat,
		Raw:          raw,
	}, nil
}
