// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_responses_client_tools.go
// @for       The tool-declaration rules the Responses-to-chat translation follows.
// @uses      encoding/json, bytes, internal/schema.
// @reason    SPEC-API-001 §7.15 serves the Responses wire, whose tool vocabulary
//
//	flattens OpenAI's `function` wrapper and adds hosted tools that carry
//	no name at all. Both are rejections upstream when they are handled
//	wrong, so the rules live in one place a test can pin.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"bytes"
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// responsesClientTools maps the tool declarations onto the chat wire's shape.
//
// A declaration that already carries OpenAI's `function` wrapper is kept as it
// is; a flattened one is wrapped. A declaration with no name at all is skipped,
// because the Responses API's hosted tools carry none and a chat upstream
// rejects a function declaration without a name.
func responsesClientTools(decls []schema.ResponsesToolDecl) []schema.Tool {
	if len(decls) == 0 {
		return nil
	}
	out := make([]schema.Tool, 0, len(decls))
	for _, decl := range decls {
		if decl.Function != nil {
			out = append(out, schema.Tool{Type: schema.BlockFunction, Function: *decl.Function})
			continue
		}
		if decl.Name == "" {
			continue
		}
		out = append(out, schema.Tool{Type: schema.BlockFunction, Function: schema.ToolFunction{
			Name:        decl.Name,
			Description: decl.Description,
			Parameters:  normalizeResponsesParameters(decl.Parameters),
			Strict:      decl.Strict,
		}})
	}
	return out
}

// normalizeResponsesParameters ensures a parameter schema carries a properties
// member, which the Responses API requires and a client may omit.
func normalizeResponsesParameters(params json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(params)) == 0 {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	decoded, ok := decodeObject(params)
	if !ok {
		return params
	}
	if stringField(decoded, "type") != "object" {
		return params
	}
	if _, present := decoded["properties"]; present {
		return params
	}
	decoded["properties"] = mustJSON(map[string]any{})
	return mustJSON(decoded)
}
