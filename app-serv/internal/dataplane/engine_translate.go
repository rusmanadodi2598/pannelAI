// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_translate.go
// @for       Turning one client request into an upstream body, and one upstream
//
//	answer back into the client's wire format.
//
// @uses      internal/schema, encoding/json.
// @reason    SPEC-API-001 §7.15 makes translation the pipeline's core, and the two
//
//	directions are decided by three facts (the client's format, the
//	upstream's format, and whether the body is streamed). Deciding them
//	through one pair of functions is what keeps the matrix from becoming
//	a branch at every call site.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// upstreamBody builds the payload the upstream receives.
//
// A target that speaks the client's own format is forwarded verbatim, with only
// the model id replaced: re-encoding a decoded DTO would silently drop every
// field the gateway does not model, which for a coding agent is most of them.
func upstreamBody(in Request, resolution Resolution) ([]byte, error) {
	sameFormat := (in.ClientFormat == schema.FormatOpenAI && resolution.Target == TargetOpenAI) ||
		(in.ClientFormat == schema.FormatAnthropic && resolution.Target == TargetClaude)

	if sameFormat {
		return replaceModel(in.Raw, resolution.UpstreamID)
	}
	return translateUpstreamBody(in, resolution)
}

// replaceModel swaps only the model member of a forwarded body, leaving every
// other byte as the client sent it.
func replaceModel(raw []byte, upstreamModel string) ([]byte, error) {
	body, ok := decodeObject(raw)
	if !ok {
		return nil, dataPlaneError(CodeValidation, "the request body could not be read")
	}
	body["model"] = mustJSON(upstreamModel)
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, wrapDataPlaneError(CodeValidation, "the request body could not be re-encoded", err)
	}
	return encoded, nil
}

// translateUpstreamBody translates between the client's format and the
// upstream's, in the direction the pair implies.
func translateUpstreamBody(in Request, resolution Resolution) ([]byte, error) {
	switch in.ClientFormat {
	case schema.FormatAnthropic:
		if in.Messages == nil {
			return nil, dataPlaneError(CodeValidation, "the request body could not be read")
		}
		switch resolution.Target {
		case TargetOpenAI:
			return encode(ClaudeToOpenAI(*in.Messages, resolution.UpstreamID, in.Stream))
		case TargetResponses:
			return encode(ClaudeToResponses(*in.Messages, resolution.UpstreamID, in.Stream))
		}
	case schema.FormatOpenAI:
		if in.Chat == nil {
			return nil, dataPlaneError(CodeValidation, "the request body could not be read")
		}
		switch resolution.Target {
		case TargetClaude:
			return encode(OpenAIToClaude(*in.Chat, resolution.UpstreamID, in.Stream))
		case TargetResponses:
			return encode(OpenAIToResponses(*in.Chat, resolution.UpstreamID, in.Stream))
		}
	}
	return nil, dataPlaneError(CodeProviderNotRoutable,
		"the gateway cannot translate "+string(in.ClientFormat)+" into "+resolution.Target)
}

// readUsage extracts the accounting a non-streamed answer reported, in whichever
// format the upstream wrote it.
func readUsage(raw []byte, upstreamTarget string) *schema.Usage {
	body, ok := decodeObject(raw)
	if !ok {
		return nil
	}
	usage, ok := objectField(body, "usage")
	if !ok {
		return nil
	}
	switch upstreamTarget {
	case TargetClaude:
		parsed := ClaudeUsageToOpenAI(claudeUsageFromObject(usage))
		return &parsed
	case TargetResponses:
		return responsesUsageFromObject(usage)
	default:
		return openAIUsageFromObject(usage)
	}
}

// claudeAnswer translates a non-streamed upstream answer into the Anthropic
// wire, whichever format the upstream wrote it in.
func claudeAnswer(raw []byte, resolution Resolution) (schema.MessagesResponse, error) {
	if resolution.Target == TargetResponses {
		return ResponsesToClaudeResponse(raw, resolution.ModelID)
	}
	return OpenAIToClaudeResponse(raw, resolution.ModelID)
}

// openAIAnswer translates a non-streamed upstream answer into the OpenAI wire,
// whichever format the upstream wrote it in.
func openAIAnswer(raw []byte, resolution Resolution, created int64) (schema.ChatCompletionResponse, error) {
	if resolution.Target == TargetResponses {
		return ResponsesToOpenAIResponse(raw, resolution.ModelID, created)
	}
	return ClaudeToOpenAIResponse(raw, resolution.ModelID, created)
}

// encode marshals a translated payload, reporting a failure as a client error
// rather than an internal one: the payload is built from the client's own request,
// so a value json cannot render came from that request.
func encode(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, wrapDataPlaneError(CodeValidation, "the request could not be translated", err)
	}
	return encoded, nil
}
