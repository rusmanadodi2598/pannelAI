// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_gemini_turns.go
// @for       The turn shape Gemini reads as a conversation: merged adjacent turns, the filler that opens a model-first history, and the result that closes a call nobody answered.
// @uses      regexp.
// @reason    The rules are only visible once the whole history is built, so they run as one pass over the walk's turns rather than per message: parallel tool results arrive as one message per call and read as a history that never alternated, and a request ending on the model's own call is refused outright. The reference folds the same three into normalizeGeminiContents.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-18
package dataplane

import "regexp"

// The two fillers stand in for text the client never sent. They have to be text:
// a turn with no parts is dropped by the same walk that is about to add one.
const (
	geminiOpeningTurnText = "..."
	geminiContinuedText   = "Continue."
)

// geminiResponsesLastPattern matches the models that reject a merged user turn
// whose last part is not a functionResponse. The reference gates the reorder on
// exactly this family (providers/capabilities.js, isGemini4Model) rather than
// applying it everywhere, so the reading of older models stays the vendor's.
var geminiResponsesLastPattern = regexp.MustCompile(`(?i)(^|/)gemini-4([-.]|$)`)

// mergeGeminiTurns collapses the walk's per-message turns into the conversation
// Gemini reads: adjacent turns of one role become one turn, and the history opens
// with a user turn rather than with the model's own words.
//
// Two of these are the shape a client's history arrives in, not a vendor quirk:
// parallel tool results are one message per call, and a caller that sends only the
// assistant turn it wants continued starts on role "model".
func mergeGeminiTurns(turns []GeminiContent, model string) []GeminiContent {
	merged := make([]GeminiContent, 0, len(turns))
	for _, turn := range turns {
		if turn.Role == "" || len(turn.Parts) == 0 {
			continue
		}
		if last := len(merged) - 1; last >= 0 && merged[last].Role == turn.Role {
			merged[last].Parts = append(merged[last].Parts, turn.Parts...)
			if geminiResponsesLastPattern.MatchString(model) {
				merged[last].Parts = geminiResponsesLast(merged[last].Parts)
			}
			continue
		}
		merged = append(merged, GeminiContent{
			Role:  turn.Role,
			Parts: append([]GeminiPart(nil), turn.Parts...),
		})
	}
	if len(merged) > 0 && merged[0].Role != RoleUser {
		merged = append([]GeminiContent{{
			Role:  RoleUser,
			Parts: []GeminiPart{{Text: geminiOpeningTurnText}},
		}}, merged...)
	}
	return appendClosingTurn(merged)
}

// appendClosingTurn answers a call the history left standing: Gemini requires a
// functionResponse for every functionCall, so a request ending on the model's own
// call is refused rather than continued. Only the calls go unanswered this way, so
// only they get the filler, and a text-only trailing turn is left as the caller
// wrote it.
func appendClosingTurn(turns []GeminiContent) []GeminiContent {
	if len(turns) == 0 || turns[len(turns)-1].Role != RoleModel {
		return turns
	}
	calls := make([]GeminiPart, 0, 2)
	for _, part := range turns[len(turns)-1].Parts {
		if part.FunctionCall == nil {
			continue
		}
		calls = append(calls, GeminiPart{FunctionResponse: &GeminiFunctionResp{
			ID:       part.FunctionCall.ID,
			Name:     part.FunctionCall.Name,
			Response: map[string]any{"result": geminiContinuedText},
		}})
	}
	if len(calls) == 0 {
		return turns
	}
	return append(turns, GeminiContent{Role: RoleUser, Parts: calls})
}

// geminiResponsesLast moves a turn's functionResponses after its text parts, which
// is the order the models matching geminiResponsesLastPattern accept.
func geminiResponsesLast(parts []GeminiPart) []GeminiPart {
	responses := make([]GeminiPart, 0, len(parts))
	rest := make([]GeminiPart, 0, len(parts))
	for _, part := range parts {
		if part.FunctionResponse != nil {
			responses = append(responses, part)
			continue
		}
		rest = append(rest, part)
	}
	return append(rest, responses...)
}
