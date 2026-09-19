// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_answer.go
// @for       Converting one non-streamed upstream answer into the client's wire
//
//	format.
//
// @uses      internal/schema, io.
// @reason    SPEC-API-001 §7.15 serves three client wires from three upstream
//
//	formats, so the non-streamed answer has its own pair of decisions.
//	Keeping it out of the stream file holds both inside the AGENTS.md
//	§1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"io"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// translateAnswer converts a non-streamed upstream answer into the client's wire
// format and reads its accounting.
func (e *Engine) translateAnswer(upstream *Upstream, resolution Resolution, in Request) ([]byte, *schema.Usage, error) {
	raw, err := io.ReadAll(upstream.Body)
	if err != nil {
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "the upstream response could not be read", err)
	}
	usage := readUsage(raw, resolution.Target)

	switch in.ClientFormat {
	case schema.FormatAnthropic:
		if resolution.Target == TargetClaude {
			return raw, usage, nil
		}
		translated, err := claudeAnswer(raw, resolution)
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	case schema.FormatOpenAIResponses:
		translated, err := responsesClientAnswer(raw, resolution, e.clock().Unix())
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	default:
		if resolution.Target == TargetOpenAI {
			return raw, usage, nil
		}
		translated, err := openAIAnswer(raw, resolution, e.clock().Unix())
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	}
}
