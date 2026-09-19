// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_fusion_fixture_test.go
// @for       The fusion fixture: an upstream that records every panel and judge
//
//	call, a sink that keeps the frames, and the request builders.
//
// @uses      testing, net/http, net/http/httptest, encoding/json, context,
//
//	sync, internal/domain, internal/registry, internal/schema.
//
// @reason    A fusion test has to prove who was asked what — which models were
//
//	consulted, whether they streamed, and what the judge was told — so
//	the recording stand-in is shared wiring rather than per-test trivia.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// fusionCall is one request the stand-in upstream received.
type fusionCall struct {
	// Model is the upstream model id the request named.
	Model string
	// Stream reports whether the caller asked for SSE.
	Stream bool
	// Tools reports whether the body declared tools.
	Tools bool
	// LastTurn is the text of the final message, which for a judge call is the
	// appended synthesis directive.
	LastTurn string
}

// fusionUpstream records the calls the panel and the judge made, in arrival
// order. The mutex is what makes it safe under the fan-out's concurrency.
type fusionUpstream struct {
	mu    sync.Mutex
	calls []fusionCall
}

func (u *fusionUpstream) record(call fusionCall) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls = append(u.calls, call)
}

// snapshot returns a copy, so an assertion never reads a slice being appended.
func (u *fusionUpstream) snapshot() []fusionCall {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]fusionCall(nil), u.calls...)
}

// callsFor returns the calls one model received.
func (u *fusionUpstream) callsFor(model string) []fusionCall {
	var found []fusionCall
	for _, call := range u.snapshot() {
		if call.Model == model {
			found = append(found, call)
		}
	}
	return found
}

// newFusionServer answers every model with a completion that names it, streams
// when asked to, and fails for the model named "broken".
func newFusionServer(t *testing.T, upstream *fusionUpstream) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model    string            `json:"model"`
			Stream   bool              `json:"stream"`
			Tools    []json.RawMessage `json:"tools"`
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("upstream received an undecodable body: %v", err)
		}
		last := ""
		if len(body.Messages) > 0 {
			last = strings.Trim(string(body.Messages[len(body.Messages)-1].Content), `"`)
		}
		upstream.record(fusionCall{Model: body.Model, Stream: body.Stream, Tools: body.Tools != nil, LastTurn: last})

		if body.Model == "broken" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"the panel member is down"}}`))
			return
		}
		if body.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			writeFusionChunk(w, body.Model, `"content":"judged answer"`, "")
			writeFusionChunk(w, body.Model, ``, "stop")
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-` + body.Model + `","object":"chat.completion","created":1,` +
			`"model":"` + body.Model + `","choices":[{"index":0,"message":{"role":"assistant","content":"answer from ` +
			body.Model + `"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// writeFusionChunk writes one OpenAI SSE chunk carrying the given delta members
// and finish reason, where an empty reason is the null a content chunk sends.
func writeFusionChunk(w http.ResponseWriter, model, delta, finish string) {
	reason := "null"
	if finish != "" {
		reason = `"` + finish + `"`
	}
	_, _ = w.Write([]byte("data: " + `{"id":"chatcmpl-` + model + `","object":"chat.completion.chunk",` +
		`"created":1,"model":"` + model + `","choices":[{"index":0,"delta":{` + delta + `},` +
		`"finish_reason":` + reason + `}]}` + "\n\n"))
}

// fusionProviders returns one OpenAI-format registry entry per provider, all
// pointing at the stand-in upstream.
func fusionProviders(url string, ids ...string) []registry.Provider {
	providers := make([]registry.Provider, 0, len(ids))
	for _, id := range ids {
		providers = append(providers, relayProvider(id, url))
	}
	return providers
}

// fusionRequest is a client request naming the combo, with the decoded body and
// the raw body kept in step the way the handler delivers them: a member the
// struct carries is a member the body carries, or a same-format call would
// forward a request the client never sent.
func fusionRequest(model string, tools, stream bool) Request {
	chat := &schema.ChatRequest{
		Model: model,
		Messages: []schema.ChatMessage{{
			Role: schema.RoleUser, Content: schema.MessageContent{Text: "ping"},
		}},
	}
	raw := `{"model":"` + model + `","messages":[{"role":"user","content":"ping"}]`
	if tools {
		chat.Tools = []schema.Tool{{Type: "function", Function: schema.ToolFunction{Name: "lookup"}}}
		raw += `,"tools":[{"type":"function","function":{"name":"lookup"}}]`
	}
	if stream {
		chat.Stream = true
		raw += `,"stream":true`
	}
	return Request{
		Route:        RouteChatCompletions,
		ClientFormat: schema.FormatOpenAI,
		Model:        model,
		Stream:       stream,
		Chat:         chat,
		Raw:          []byte(raw + "}"),
	}
}

// recordSink keeps the frames a streamed answer wrote, so a test can assert the
// client received the judge's stream rather than a panel member's.
type recordSink struct {
	mu     sync.Mutex
	frames []string
	flush  int
}

func (s *recordSink) WriteFrame(frame []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frames = append(s.frames, string(frame))
	return nil
}

func (s *recordSink) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flush++
}

func (s *recordSink) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.frames, "\n")
}
