// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_gemini_wire_test.go
// @for       One Gemini call as it actually leaves and arrives on the socket.
// @uses      context, encoding/json, io, net/http, net/http/httptest, strings, testing, internal/provider, internal/registry, internal/schema.
// @reason    A unit test can show what a translator produced; only a wire test shows the URL the vendor was dialed at, the body it was handed, the header the credential rode, and the numbers the caller was billed. Those four are what a format can get wrong while every function in the chain still reports success, and the usage half of it is the silent one: a body read with the wrong key answers 200 with zero tokens.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package dataplane

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// geminiWireCall is one request as the upstream stand-in received it.
type geminiWireCall struct {
	path   string
	body   string
	header http.Header
}

const (
	geminiWireAnswer = `{"candidates":[{"content":{"role":"model","parts":[{"text":"pong"}]},` +
		`"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":5,"totalTokenCount":16}}`

	geminiWireStream = "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"po\"}]}}]}\n\n" +
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ng\"}]},\"finishReason\":\"STOP\"}]}\n\n" +
		"data: {\"usageMetadata\":{\"promptTokenCount\":7,\"candidatesTokenCount\":2,\"totalTokenCount\":9}}\n\n"
)

// geminiWireUpstream answers the generateContent call and records what arrived.
func geminiWireUpstream(t *testing.T, captured *geminiWireCall, stream bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("the upstream body could not be read: %v", err)
		}
		captured.path = r.URL.Path
		captured.body = string(raw)
		captured.header = r.Header.Clone()

		if !strings.Contains(r.URL.Path, ":generateContent") && !strings.Contains(r.URL.Path, ":streamGenerateContent") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"the action is not a generateContent one"}}`))
			return
		}
		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(geminiWireStream))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(geminiWireAnswer))
	}))
	t.Cleanup(server.Close)
	return server
}

// geminiWireEntry mirrors the embedded entry's shape against the stand-in: the
// format, the path-shaped base, and the header this vendor reads a key from.
func geminiWireEntry(url string) registry.Provider {
	return registry.Provider{
		ID: "gemini", Priority: 1, Category: "freeTier", PassthroughModels: true,
		Transport: registry.Transport{
			BaseURL: url + "/v1beta/models",
			Format:  "gemini",
			Auth: registry.AuthConfig{
				APIKey: &registry.AuthScheme{Header: "x-goog-api-key", Scheme: "raw"},
			},
		},
	}
}

func geminiWireRequest(t *testing.T, model string, stream bool) Request {
	t.Helper()
	raw := `{"model":"` + model + `","messages":[` +
		`{"role":"system","content":"Reply with one word only."},` +
		`{"role":"user","content":"ping"}]}`

	// The handler decodes the client body before the engine runs, so a wire test
	// that filled only Raw would translate an empty conversation: what the engine
	// reads is Chat, and Raw is what it echoes for members it does not model.
	var chat schema.ChatRequest
	if err := json.Unmarshal([]byte(raw), &chat); err != nil {
		t.Fatalf("decoding the client body: %v", err)
	}
	if len(chat.Messages) != 2 {
		t.Fatalf("decoded messages = %d, want the two the client sent", len(chat.Messages))
	}

	in := relayRequest(model)
	in.Chat = &chat
	in.Stream = stream
	in.IncludeUsage = true
	in.Raw = []byte(raw)
	return in
}

// TestGeminiWire_OutboundRequestIsTheCallThisVendorAccepts pins the three things
// the vendor reads before it looks at the prompt: the addressed path, the payload
// shape, and the header carrying the key.
func TestGeminiWire_OutboundRequestIsTheCallThisVendorAccepts(t *testing.T) {
	var captured geminiWireCall
	server := geminiWireUpstream(t, &captured, false)
	entry := geminiWireEntry(server.URL)
	engine := newForcedStreamEngine(t, server.URL, entry, provider.NewGemini(entry))

	if _, err := engine.Relay(context.Background(), geminiWireRequest(t, "gemini/gemini-2.5-flash", false), nil); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	if want := "/v1beta/models/gemini-2.5-flash:generateContent"; captured.path != want {
		t.Fatalf("upstream path = %q, want %q: the model is a path segment here", captured.path, want)
	}
	if !strings.Contains(captured.body, `"contents"`) {
		t.Fatalf("upstream body = %s, want the Gemini contents vocabulary", captured.body)
	}
	if !strings.Contains(captured.body, `"systemInstruction"`) {
		t.Fatalf("upstream body = %s, want the system prompt hoisted out of contents", captured.body)
	}
	// The model belongs in the path: a body member naming it would be a second
	// source for the same fact, and Gemini reads only the path.
	if strings.Contains(captured.body, `"model"`) {
		t.Fatalf("upstream body = %s, want no model member", captured.body)
	}
	if got := captured.header.Get("x-goog-api-key"); got == "" {
		t.Fatal("x-goog-api-key is absent, want the stored key on the header this vendor reads")
	}
	if got := captured.header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want no bearer for a key entry", got)
	}
}

// TestGeminiWire_AnswerReachesTheClientWithItsNumbers is the regression this whole
// stage exists to close. Every per-target switch in the engine ends in a default
// that assumes an OpenAI upstream, so an unmapped fourth target would answer 200
// with an empty completion and, through the usage key mismatch, zero tokens billed.
func TestGeminiWire_AnswerReachesTheClientWithItsNumbers(t *testing.T) {
	var captured geminiWireCall
	server := geminiWireUpstream(t, &captured, false)
	entry := geminiWireEntry(server.URL)
	engine := newForcedStreamEngine(t, server.URL, entry, provider.NewGemini(entry))

	outcome, err := engine.Relay(context.Background(), geminiWireRequest(t, "gemini/gemini-2.5-flash", false), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	var answer schema.ChatCompletionResponse
	if err := json.Unmarshal(outcome.Body, &answer); err != nil {
		t.Fatalf("the answer is not a chat completion: %v (body=%s)", err, outcome.Body)
	}
	if len(answer.Choices) != 1 || answer.Choices[0].Message.Content.Text != "pong" {
		t.Fatalf("answer = %s, want the candidate text as the message", outcome.Body)
	}
	if answer.Choices[0].FinishReason != FinishStop {
		t.Fatalf("finish_reason = %q, want STOP mapped onto OpenAI's stop", answer.Choices[0].FinishReason)
	}
	if outcome.Usage == nil {
		t.Fatal("Outcome.Usage = nil, want the usageMetadata numbers billed")
	}
	if outcome.Usage.PromptTokens != 11 || outcome.Usage.CompletionTokens != 5 || outcome.Usage.TotalTokens != 16 {
		t.Fatalf("usage = %+v, want 11/5/16 read from usageMetadata", outcome.Usage)
	}
}

// TestGeminiWire_StreamReachesTheClientFrameByFrame pins the streamed half, where
// the same default would have forwarded a Gemini payload as if it were an OpenAI
// chunk: the deltas arrive in the client's vocabulary, and the terminal frames are
// the usage chunk and one finish, not a `[DONE]` this vendor never sends.
func TestGeminiWire_StreamReachesTheClientFrameByFrame(t *testing.T) {
	var captured geminiWireCall
	server := geminiWireUpstream(t, &captured, true)
	entry := geminiWireEntry(server.URL)
	engine := newForcedStreamEngine(t, server.URL, entry, provider.NewGemini(entry))

	sink := &recordingSink{}
	outcome, err := engine.Relay(context.Background(), geminiWireRequest(t, "gemini/gemini-2.5-flash", true), sink)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if !outcome.Streamed {
		t.Fatal("Outcome.Streamed = false, want the answer delivered as frames")
	}
	if want := "/v1beta/models/gemini-2.5-flash:streamGenerateContent"; !strings.HasPrefix(captured.path, want) {
		t.Fatalf("upstream path = %q, want it to start at the streaming action %q", captured.path, want)
	}
	if captured.body == "" || !strings.Contains(captured.body, `"contents"`) {
		t.Fatalf("upstream body = %s, want the Gemini contents vocabulary on the streamed call too", captured.body)
	}

	frames := strings.Join(frameStrings(sink.frames), "")
	if !strings.Contains(frames, `"content":"po"`) || !strings.Contains(frames, `"content":"ng"`) {
		t.Fatalf("frames = %s, want both deltas forwarded as OpenAI content", frames)
	}
	if !strings.Contains(frames, `"finish_reason":"stop"`) {
		t.Fatalf("frames = %s, want one finish frame", frames)
	}
	if strings.Count(frames, `"finish_reason":"stop"`) != 1 {
		t.Fatalf("frames = %s, want exactly one finish reason", frames)
	}
	if !strings.Contains(frames, `"prompt_tokens":7`) {
		t.Fatalf("frames = %s, want the usage the vendor reported on its own frame", frames)
	}
	// The vendor ends its stream by closing it. A client on the OpenAI wire waits
	// for the terminal marker, so the gateway owes it even though no frame carried it.
	if !strings.Contains(frames, "[DONE]") {
		t.Fatalf("frames = %s, want the terminal marker the upstream never sent", frames)
	}
}
