// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_gemini_calls_test.go
// @for       The tool-call agreement a Gemini history has to keep: ids unique across the conversation, results paired to their own call, turn order the vendor accepts, and the signature a replayed call carries.
// @uses      strings, testing, internal/schema.
// @reason    Every rule here is a request Gemini refuses outright rather than answers wrong: a repeated functionCall id fails the whole call (400 INVALID_ARGUMENT), a merged turn that ends on text fails on the models that want the results last, and a replayed call with no thoughtSignature fails on Gemini 3. None of it shows up in a single-turn test, so each is pinned on a history of at least two turns.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-18
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

func TestGeminiHistoryKeepsRepeatedCallIDsApart(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "two turns"}},
		{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{{
			ID: "call_1", Function: schema.FunctionCall{Name: "read", Arguments: `{"p":"a"}`},
		}}},
		{Role: schema.RoleTool, ToolCallID: "call_1", Content: schema.MessageContent{Text: `{"ok":true}`}},
		{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{{
			ID: "call_1", Function: schema.FunctionCall{Name: "write", Arguments: `{"p":"b"}`},
		}}},
		{Role: schema.RoleTool, ToolCallID: "call_1", Content: schema.MessageContent{Text: `{"ok":false}`}},
	}}
	out := OpenAIToGemini(request, "gemini-2.5-flash", geminiProductAPI)

	first, second := out.Contents[1].Parts[0], out.Contents[3].Parts[0]
	if first.FunctionCall.ID == "" || first.FunctionCall.ID == second.FunctionCall.ID {
		t.Fatalf("call ids = %q / %q, want two distinct ids where the client repeated one",
			first.FunctionCall.ID, second.FunctionCall.ID)
	}
	// Each result answers the call that asked for it, name included: a map keyed
	// on the id alone would rename the first turn's response after the second.
	calls := map[string]*GeminiFunctionCall{
		first.FunctionCall.ID:  first.FunctionCall,
		second.FunctionCall.ID: second.FunctionCall,
	}
	for _, turn := range out.Contents {
		for _, part := range turn.Parts {
			if part.FunctionResponse == nil {
				continue
			}
			declared, ok := calls[part.FunctionResponse.ID]
			if !ok {
				t.Fatalf("response id = %q, want one this history's calls carried", part.FunctionResponse.ID)
			}
			if part.FunctionResponse.Name != declared.Name {
				t.Fatalf("response %q names %q, want the name its own call declared (%q)",
					part.FunctionResponse.ID, part.FunctionResponse.Name, declared.Name)
			}
		}
	}
}

func TestGeminiHistorySignsOneCallPerModelTurn(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "go"}},
		{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{
			{ID: "a", Function: schema.FunctionCall{Name: "read", Arguments: `{}`}},
			{ID: "b", Function: schema.FunctionCall{Name: "list", Arguments: `{}`}},
		}},
		{Role: schema.RoleTool, ToolCallID: "a", Content: schema.MessageContent{Text: "1"}},
		{Role: schema.RoleTool, ToolCallID: "b", Content: schema.MessageContent{Text: "2"}},
		{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{{
			ID: "c", Function: schema.FunctionCall{Name: "read", Arguments: `{}`},
		}}},
	}}
	out := OpenAIToGemini(request, "gemini-2.5-flash", geminiProductAntigravity)

	turns := make([][]GeminiPart, 0, 2)
	for _, content := range out.Contents {
		if content.Role == RoleModel {
			turns = append(turns, content.Parts)
		}
	}
	if len(turns) != 2 {
		t.Fatalf("model turns = %d, want the two the caller sent", len(turns))
	}
	if turns[0][0].ThoughtSignature != geminiThoughtSignatureAPI {
		t.Fatalf("first call of the turn unsigned, want the product's signature on it")
	}
	if turns[0][1].ThoughtSignature != "" {
		t.Fatalf("sibling call signed, want only the turn's first call carrying one")
	}
	if turns[1][0].ThoughtSignature != geminiThoughtSignatureAPI {
		t.Fatal("second model turn unsigned, want one signature per turn rather than per request")
	}
}

func TestGeminiProductSignaturesDifferPerProduct(t *testing.T) {
	cases := []struct {
		providerID string
		want       geminiProduct
	}{
		{providerID: "gemini", want: geminiProductAPI},
		{providerID: "antigravity", want: geminiProductAntigravity},
		{providerID: "gemini-cli", want: geminiProductGeminiCLI},
		{providerID: "custom-node", want: geminiProductAPI},
	}
	for _, tc := range cases {
		if got := geminiProductFor(tc.providerID); got != tc.want {
			t.Fatalf("geminiProductFor(%q) = %q, want %q", tc.providerID, got, tc.want)
		}
	}
	if geminiProductGeminiCLI.thoughtSignature() == geminiProductAntigravity.thoughtSignature() {
		t.Fatal("the two Cloud Code Assist products share a signature, want one per product")
	}
}

// TestGeminiHistoryMergesParallelResults pins the turn shape a client's history
// arrives in: two tool messages answer one assistant turn, and Gemini reads two
// consecutive user turns as a conversation that never alternated.
func TestGeminiHistoryMergesParallelResults(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "go"}},
		{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{
			{ID: "a", Function: schema.FunctionCall{Name: "read", Arguments: `{}`}},
			{ID: "b", Function: schema.FunctionCall{Name: "list", Arguments: `{}`}},
		}},
		{Role: schema.RoleTool, ToolCallID: "a", Content: schema.MessageContent{Text: "1"}},
		{Role: schema.RoleTool, ToolCallID: "b", Content: schema.MessageContent{Text: "2"}},
	}}
	out := OpenAIToGemini(request, "gemini-2.5-flash", geminiProductAPI)

	if len(out.Contents) != 3 {
		t.Fatalf("contents = %d, want the two results merged into one user turn", len(out.Contents))
	}
	results := out.Contents[2]
	if results.Role != RoleUser || len(results.Parts) != 2 {
		t.Fatalf("result turn = %+v, want one user turn holding both responses", results)
	}
}

// TestGeminiHistoryOpensOnTheUser answers a call the caller left standing: a
// history that starts on the model's own call, or ends on one, is refused rather
// than continued.
func TestGeminiHistoryOpensOnTheUserAndClosesACall(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleAssistant, Content: schema.MessageContent{Text: "thinking"}, ToolCalls: []schema.ToolCall{
			{ID: "a", Function: schema.FunctionCall{Name: "read", Arguments: `{}`}},
		}},
	}}
	out := OpenAIToGemini(request, "gemini-2.5-flash", geminiProductAPI)

	if len(out.Contents) != 3 {
		t.Fatalf("contents = %d, want a leading user turn and a closing result", len(out.Contents))
	}
	if out.Contents[0].Role != RoleUser || out.Contents[0].Parts[0].Text != geminiOpeningTurnText {
		t.Fatalf("opening turn = %+v, want the user filler a model-first history needs", out.Contents[0])
	}
	closing := out.Contents[2].Parts[0].FunctionResponse
	if closing == nil || closing.Name != "read" {
		t.Fatalf("closing turn = %+v, want the dangling call answered", out.Contents[2])
	}
	if got, ok := closing.Response["result"].(string); !ok || got != geminiContinuedText {
		t.Fatalf("closing result = %+v, want the filler the caller never sent", closing.Response)
	}
}

// TestGeminiHistoryPutsResultsLastForGemini4 holds the order the newer models
// require inside a merged turn; the reference gates the same reorder on this
// model family only, so a Gemini 2.5 request keeps the order the client sent.
func TestGeminiHistoryPutsResultsLastForGemini4(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "go"}},
		{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{{
			ID: "a", Function: schema.FunctionCall{Name: "read", Arguments: `{}`},
		}}},
		{Role: schema.RoleTool, ToolCallID: "a", Content: schema.MessageContent{Text: "1"}},
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "and now?"}},
	}}

	for _, model := range []string{"gemini-2.5-flash", "gemini-4-pro"} {
		out := OpenAIToGemini(request, model, geminiProductAPI)
		merged := out.Contents[len(out.Contents)-1].Parts
		if len(merged) != 2 {
			t.Fatalf("%s: merged turn = %d parts, want the result and the text in one turn", model, len(merged))
		}
		wantResponsesLast := strings.HasPrefix(model, "gemini-4")
		if got := merged[1].FunctionResponse != nil; got != wantResponsesLast {
			t.Fatalf("%s: merged turn order = %+v, want the functionResponse last only where the model requires it", model, merged)
		}
	}
}

func TestGeminiHistoryNamesAnUnmatchedResultByItsID(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "go"}},
		{Role: schema.RoleTool, ToolCallID: "call_orphan", Content: schema.MessageContent{Text: `{"ok":true}`}},
	}}
	out := OpenAIToGemini(request, "gemini-2.5-flash", geminiProductAPI)

	response := geminiFunctionResponses(out.Contents)
	if len(response) != 1 {
		t.Fatalf("functionResponses = %d, want the one orphaned result", len(response))
	}
	if response[0].Name != "call_orphan" {
		t.Fatalf("orphaned result = %+v, want the id as the name rather than one parsed out of it", response[0])
	}
	if response[0].ID != "call_orphan" {
		t.Fatalf("orphaned result id = %q, want the id the client sent", response[0].ID)
	}
}

// geminiFunctionResponses collects every result part a built request carries, so a
// test can follow one through whichever turn the merge put it in.
func geminiFunctionResponses(turns []GeminiContent) []*GeminiFunctionResp {
	found := make([]*GeminiFunctionResp, 0, len(turns))
	for _, turn := range turns {
		for _, part := range turn.Parts {
			if part.FunctionResponse != nil {
				found = append(found, part.FunctionResponse)
			}
		}
	}
	return found
}
