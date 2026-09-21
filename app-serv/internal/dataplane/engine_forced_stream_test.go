// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_forced_stream_test.go
// @for       Serving a non-streaming client from a provider that only answers a
//
//	stream.
//
// @uses      testing, context, encoding/json, net/http, net/http/httptest,
//
//	internal/domain, internal/provider, internal/registry, internal/schema.
//
// @reason    OpenCode Free refuses a non-streaming request with 403, so a client
//
//	that asked for one JSON body has to be served from the stream the
//	provider does send. The fold is the core's job, not the connector's:
//	a connector owns the outbound shape, and turning an answer back into
//	the client's wire is what the translation layer already does. Both
//	upstream wires the gateway can receive are covered, because the
//	free tier forces a stream on each.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-21
package dataplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// forcedStreamConnector is a connector that only answers a stream, so the core
// has to fold the stream back into the single body a non-streaming client asked
// for. It answers ForcesStream without the transport-level declaration, which is
// the seam a connector owns.
type forcedStreamConnector struct {
	provider.Base
	url string
}

func (c *forcedStreamConnector) Endpoint(provider.Request, provider.Credential) (string, error) {
	return c.url, nil
}

func (c *forcedStreamConnector) ApplyAuth(*http.Request, provider.Credential) error { return nil }

// ForcesStream implements the optional seam the core reads.
func (c *forcedStreamConnector) ForcesStream() bool { return true }

// forcedStreamRegistry exposes one provider the connector is registered for.
type forcedStreamRegistry struct {
	entry registry.Provider
}

func (r forcedStreamRegistry) Provider(name string) (registry.Provider, bool) {
	if name == r.entry.ID {
		return r.entry, true
	}
	return registry.Provider{}, false
}

func (forcedStreamRegistry) Model(string, string) (registry.Model, bool) {
	return registry.Model{}, false
}

func (r forcedStreamRegistry) All() []registry.Provider { return []registry.Provider{r.entry} }

// newForcedStreamEngine wires the pipeline over a connector that forces a stream
// and an upstream stand-in that answers one.
func newForcedStreamEngine(t *testing.T, upstreamURL string, entry registry.Provider, connector provider.Plugin) *Engine {
	t.Helper()
	resolver, err := NewResolver(
		forcedStreamRegistry{entry: entry},
		relayLookup{combos: map[string]domain.Combo{}},
	)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, connector)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	repo := newMemEndpointRepo()
	repo.byProvider[entry.ID] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-"+entry.ID, entry.ID)}
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}, StickyLimit: 1})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: http.DefaultClient})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	engine, err := NewEngine(EngineDeps{Resolver: resolver, Selector: selector, Transport: transport})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return engine
}

// responsesStreamBody is the SSE a Responses-wire upstream sends for a short
// answer, ending in the terminal event that carries the complete response.
const responsesStreamBody = "event: response.created\n" +
	`data: {"type":"response.created","response":{"id":"resp_1","model":"muse"}}` + "\n\n" +
	"event: response.output_text.delta\n" +
	`data: {"type":"response.output_text.delta","delta":"PO"}` + "\n\n" +
	"event: response.output_text.delta\n" +
	`data: {"type":"response.output_text.delta","delta":"NG"}` + "\n\n" +
	"event: response.completed\n" +
	`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed",` +
	`"model":"muse","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"PONG"}]}],` +
	`"usage":{"input_tokens":570,"output_tokens":175,"total_tokens":745}}}` + "\n\n" +
	"data: [DONE]\n\n"

// chatStreamBody is the SSE a chat-wire upstream sends for the same answer, split
// across the frames a real stream uses.
const chatStreamBody = `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"big-pickle",` +
	`"choices":[{"index":0,"delta":{"role":"assistant","content":"PO"},"finish_reason":null}]}` + "\n\n" +
	`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"big-pickle",` +
	`"choices":[{"index":0,"delta":{"content":"NG"},"finish_reason":null}]}` + "\n\n" +
	`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"big-pickle",` +
	`"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
	`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"big-pickle",` +
	`"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}` + "\n\n" +
	"data: [DONE]\n\n"

// newStreamingUpstream serves one SSE body and records whether the outbound
// request asked for a stream.
func newStreamingUpstream(t *testing.T, body string, sawStream *bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var decoded struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Errorf("upstream received an undecodable body: %v", err)
		}
		*sawStream = decoded.Stream
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
}

// TestRelay_ForcedStreamServesANonStreamingClientFromResponses pins the fold for
// the Responses wire: a client that asked for one JSON body receives a complete
// chat completion assembled from the stream, including the accounting the
// terminal event reported, and the outbound request asked for a stream even
// though the client did not.
func TestRelay_ForcedStreamServesANonStreamingClientFromResponses(t *testing.T) {
	var sawStream bool
	server := newStreamingUpstream(t, responsesStreamBody, &sawStream)
	defer server.Close()

	entry := registry.Provider{
		ID: "forced-responses", Priority: 1, Category: "free", PassthroughModels: true,
		Transport: registry.Transport{BaseURL: server.URL, Format: registry.FormatOpenAIResponses},
	}
	engine := newForcedStreamEngine(t, server.URL, entry, &forcedStreamConnector{
		Base: provider.Base{ID: entry.ID, Auth: "no_auth", Format: registry.FormatOpenAIResponses},
		url:  server.URL,
	})

	in := relayRequest("forced-responses/muse")
	in.Stream = false

	outcome, err := engine.Relay(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if outcome.Streamed {
		t.Fatal("Outcome.Streamed = true, want false: the client asked for one body")
	}
	if !sawStream {
		t.Fatal("the outbound request did not ask for a stream, want the connector's forced stream")
	}

	var answer schema.ChatCompletionResponse
	if err := json.Unmarshal(outcome.Body, &answer); err != nil {
		t.Fatalf("the answer is not a chat completion: %v (body=%s)", err, outcome.Body)
	}
	if len(answer.Choices) != 1 {
		t.Fatalf("choices = %d, want one", len(answer.Choices))
	}
	if got := answer.Choices[0].Message.Content.Text; got != "PONG" {
		t.Fatalf("content = %q, want PONG", got)
	}
	if outcome.Usage == nil {
		t.Fatal("Outcome.Usage = nil, want the terminal event's accounting")
	}
	if outcome.Usage.PromptTokens != 570 || outcome.Usage.CompletionTokens != 175 {
		t.Fatalf("usage = %+v, want the upstream's 570/175", outcome.Usage)
	}
}
