// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_claude_passthrough.go
// @for       Forwarding an upstream that already speaks Anthropic, and the usage both directions fold into one accounting.
// @uses      internal/schema.
// @reason    A Claude upstream's events need no re-framing, only the client's identity and a usage total that survives being split across message_start and message_delta. The concern is separated from the state machine so the machine keeps its own file readable.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// Usage reports the accounting the stream observed, or nil when the upstream
// reported none.
func (s *ClaudeStreamState) Usage() *schema.Usage { return s.usage }

// mergeUsage folds one Anthropic usage block into the passthrough stream's
// accounting, so a message_delta that names only the output side keeps the
// prompt the message_start reported.
func (s *ClaudeStreamState) mergeUsage(usage object) {
	s.usage = mergedClaudeUsage(s.usage, usage)
}

// forwardClaude re-emits an Anthropic event the upstream already framed, keeping
// the client's identity for the opening message.
func (s *ClaudeStreamState) forwardClaude(chunk object) [][]byte {
	if eventType := stringField(chunk, "type"); eventType != "" {
		if message, ok := objectField(chunk, "message"); ok {
			if id := stringField(message, "id"); id != "" && s.ID == "" {
				s.ID = id
			}
			// message_start carries its numbers inside the message; message_delta
			// carries them at the top level. The passthrough must read both
			// places or the prompt side is never accounted for at all.
			if usage, ok := objectField(message, "usage"); ok {
				s.mergeUsage(usage)
			}
		}
		if usage, ok := objectField(chunk, "usage"); ok {
			s.mergeUsage(usage)
		}
		s.started = true
		return [][]byte{eventFrame(eventType, mustJSON(chunk))}
	}
	return nil
}
