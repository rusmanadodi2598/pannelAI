// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_decoys.go
// @for       The decoy tools the OpenCode free tier requires in every payload.
// @uses      encoding/json.
// @reason    The free tier answers 403 unless the payload declares both `bash`
//
//	and `read`, so the connector injects whichever is missing. The
//	description is the reference's own text: it tells the model the tool
//	is unavailable, which keeps a decoy from being called. They are
//	separate from the rest of the transform because the shapes differ per
//	wire and the injection is a set union over the client's own tools.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ensureOpenCodeDecoys adds whichever decoy the body is missing, in the shape the
// wire's own tool vocabulary uses: the Responses wire names the function on the
// tool itself, and the chat wire nests it under `function`.
//
// A decoy the client already declared is not duplicated, and the client's own
// tools are never removed: the model still has to be able to call what the client
// asked for.
func ensureOpenCodeDecoys(body map[string]json.RawMessage, wire string) {
	responses := wire == registry.FormatOpenAIResponses
	tools, _ := decodeOpenCodeItems(body["tools"])

	present := make(map[string]bool, len(tools))
	for _, raw := range tools {
		if name := openCodeToolName(raw); name != "" {
			present[name] = true
		}
	}
	for _, name := range openCodeDecoyNames {
		if present[name] {
			continue
		}
		tools = append(tools, openCodeDecoyTool(name, responses))
	}
	body["tools"] = mustRawList(tools)
}

// openCodeToolName reads a tool's function name in either wire shape, so a decoy
// the client declared in the other shape is still recognised as present.
func openCodeToolName(raw json.RawMessage) string {
	tool, ok := decodeOpenCodeBody(raw)
	if !ok {
		return ""
	}
	if name := stringMember(tool, "name"); name != "" {
		return name
	}
	function, ok := decodeOpenCodeBody(tool["function"])
	if !ok {
		return ""
	}
	return stringMember(function, "name")
}

// openCodeDecoyTool builds one decoy in the wire's own shape.
func openCodeDecoyTool(name string, responses bool) json.RawMessage {
	declaration := map[string]json.RawMessage{
		"name":        mustRawString(name),
		"description": mustRawString(openCodeDecoyToolDescription),
		"parameters":  json.RawMessage(`{"type":"object","properties":{}}`),
	}
	if responses {
		declaration["type"] = mustRawString("function")
		return mustRaw(declaration)
	}
	return mustRaw(map[string]json.RawMessage{
		"type":     mustRawString("function"),
		"function": mustRaw(declaration),
	})
}
