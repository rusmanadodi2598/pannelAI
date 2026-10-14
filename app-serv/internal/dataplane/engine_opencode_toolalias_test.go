// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_opencode_toolalias_test.go
// @for       The live failure end to end: a caller tool that reuses a fingerprint name comes back under the caller's own name.
// @uses      context, io, net/http, net/http/httptest, strings, testing, internal/schema.
// @reason    The failure was observed live on a real answer (`name: "read_client"`, args `{"path":"/tmp/hello.txt"}`), not in a unit shape: the alias is born in the connector and has to be undone before the client reads the frame. Driving the real pipeline proves the map survives every hop between the two, and the name is deliberately split across the stand-in's chunks so the accumulation case runs on the way.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// openCodeAliasedCallRequest is the caller's own request: a tool named exactly
// `read` with its own definition, which the fingerprint pass conceals as
// `read_client` on the way upstream.
func openCodeAliasedCallRequest(t *testing.T, stream bool) Request {
	t.Helper()
	streamed := "false"
	if stream {
		streamed = "true"
	}
	raw := []byte(`{"model":"opencode/space-bunny-free","messages":[{"role":"user","content":"read /tmp/hello.txt"}],` +
		`"tools":[{"type":"function","function":{"name":"read","description":"mine",` +
		`"parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}}}],` +
		`"stream":` + streamed + `}`)
	decoded, err := schema.DecodeChatRequest(raw)
	if err != nil {
		t.Fatalf("decoding the client body: %v", err)
	}
	return Request{
		Route: RouteChatCompletions, ClientFormat: schema.FormatOpenAI,
		Model: decoded.Model, Chat: &decoded, Raw: raw, Stream: decoded.Stream,
	}
}

// openCodeAliasedUpstream stands in for the upstream answering a call to the
// concealed name, with the name split across chunks as a live one may arrive,
// and records the outbound body so the request side can be checked beside the
// answer.
func openCodeAliasedUpstream(t *testing.T, outbound *[]byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*outbound = body
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"id":"free-1","object":"chat.completion.chunk","created":1,"model":"space-bunny-free","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_cli"}}]},"finish_reason":null}]}`,
			`data: {"id":"free-1","object":"chat.completion.chunk","created":1,"model":"space-bunny-free","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"ent","arguments":"{\"path\":\"/tmp/hello.txt\"}"}}]},"finish_reason":null}]}`,
			`data: {"id":"free-1","object":"chat.completion.chunk","created":1,"model":"space-bunny-free","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9}}`,
			"data: [DONE]\n\n",
		}, "\n\n"))
	}))
	t.Cleanup(server.Close)
	return server
}

// TestOpenCodeAliasedCall_StreamedAnswerCarriesTheCallersName reproduces the
// live failure shape: the caller declared `read`, the upstream answered the
// concealed name, and the stream the client reads names `read`. The alias
// never reaches it.
func TestOpenCodeAliasedCall_StreamedAnswerCarriesTheCallersName(t *testing.T) {
	var outbound []byte
	server := openCodeAliasedUpstream(t, &outbound)
	engine := newOpenCodeFreeEngine(t, server.URL, nil)

	sink := &recordingSink{}
	if _, err := engine.Relay(context.Background(), openCodeAliasedCallRequest(t, true), sink); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if !strings.Contains(string(outbound), `"name":"read_client"`) {
		t.Fatalf("the outbound body must carry the concealed name, got %s", outbound)
	}
	var streamed strings.Builder
	for _, frame := range sink.frames {
		streamed.Write(frame)
	}
	if !strings.Contains(streamed.String(), `"name":"read"`) {
		t.Fatalf("the client stream must name the caller's tool, got %s", streamed.String())
	}
	if strings.Contains(streamed.String(), "read_client") {
		t.Fatalf("the alias reached the client: %s", streamed.String())
	}
}

// TestOpenCodeAliasedCall_OneBodyAnswerCarriesTheCallersName is the same shape
// for a client that asked for one body: the provider forces a stream, the fold
// reassembles the call, and the body the client receives names `read`.
func TestOpenCodeAliasedCall_OneBodyAnswerCarriesTheCallersName(t *testing.T) {
	var outbound []byte
	server := openCodeAliasedUpstream(t, &outbound)
	engine := newOpenCodeFreeEngine(t, server.URL, nil)

	outcome, err := engine.Relay(context.Background(), openCodeAliasedCallRequest(t, false), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	body := string(outcome.Body)
	if !strings.Contains(body, `"name":"read"`) || strings.Contains(body, "read_client") {
		t.Fatalf("body = %s, want the caller's name and no alias", body)
	}
}
