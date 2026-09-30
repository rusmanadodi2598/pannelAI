// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_codebuddy_wire_test.go
// @for       The body that actually leaves the gateway for a CodeBuddy region.
// @uses      context, encoding/json, net/http, net/http/httptest, testing, internal/provider, internal/registry, internal/schema.
// @reason    The vendor answers a plain OpenAI message list with `11101 invalid request`, and the
//
//	rewrite that avoids it belongs to the connector. A unit test can show what the
//	connector produced; only a wire test shows what the gateway put on the socket after
//	translation and the forced stream were applied, which is the body the vendor either
//	accepts or refuses.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package dataplane

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// codeBuddyWireUpstream answers the chat stream a CodeBuddy region sends and keeps
// the exact body it received, so the assertions read the outbound request rather
// than a connector's own opinion about it.
func codeBuddyWireUpstream(t *testing.T, captured *[]byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("the upstream body could not be read: %v", err)
		}
		*captured = raw
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(chatStreamBody))
	}))
	t.Cleanup(server.Close)
	return server
}

// TestCodeBuddyWire_OutboundBodyCarriesTheShapeTheVendorAccepts pins what leaves the
// gateway, after translation and the forced stream, because that byte-for-byte body
// is what the vendor reads as `11101` or as a question.
func TestCodeBuddyWire_OutboundBodyCarriesTheShapeTheVendorAccepts(t *testing.T) {
	var captured []byte
	server := codeBuddyWireUpstream(t, &captured)
	entry := relayProvider("codebuddy-intl", server.URL)

	engine := newForcedStreamEngine(t, server.URL, entry, provider.NewCodeBuddy(entry))
	in := relayRequest("codebuddy-intl/glm-5.2")
	in.Raw = []byte(`{"model":"codebuddy-intl/glm-5.2","messages":[` +
		`{"role":"system","content":"Reply with one word only."},` +
		`{"role":"user","content":"ping"}]}`)
	in.Stream = false

	outcome, err := engine.Relay(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Relay() error = %v, want the shaped body to be accepted", err)
	}
	if outcome.Streamed {
		t.Fatal("Outcome.Streamed = true, want false: the client asked for one JSON body and the fold is the core's job")
	}

	var body struct {
		Stream   bool `json:"stream"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("the upstream received a body that is not the expected shape: %v (%s)", err, captured)
	}

	if !body.Stream {
		t.Fatalf("stream = false: this vendor refuses a non-streaming request, and its connector declares the stream")
	}
	if len(body.Messages) != 2 {
		t.Fatalf("messages = %d, want one leading system turn and the caller's turn: %s", len(body.Messages), captured)
	}

	var leading string
	if err := json.Unmarshal(body.Messages[0].Content, &leading); err != nil {
		t.Fatalf("the leading system content is not a string: %s", body.Messages[0].Content)
	}
	if leading != "You are CodeBuddy Code.\n\nReply with one word only." {
		t.Fatalf("leading system = %q, want the vendor's prompt carrying the caller's instruction rather than dropping it", leading)
	}
	if body.Messages[1].Role != "user" {
		t.Fatalf("messages[1].role = %q, want user", body.Messages[1].Role)
	}
	var blocks []map[string]string
	if err := json.Unmarshal(body.Messages[1].Content, &blocks); err != nil {
		t.Fatalf("user content = %s, want typed blocks the vendor can read", body.Messages[1].Content)
	}
	if len(blocks) != 1 || blocks[0]["type"] != "text" || blocks[0]["text"] != "ping" {
		t.Fatalf("user blocks = %v, want one text block carrying the caller's question", blocks)
	}
	if outcome.ProviderID != "codebuddy-intl" {
		t.Fatalf("outcome.ProviderID = %q, want the region the request addressed", outcome.ProviderID)
	}
	var answer schema.ChatCompletionResponse
	if err := json.Unmarshal(outcome.Body, &answer); err != nil {
		t.Fatalf("the answer is not a chat completion: %v (body=%s)", err, outcome.Body)
	}
	if len(answer.Choices) != 1 || answer.Choices[0].Message.Content.Text != "PONG" {
		t.Fatalf("answer = %s, want the folded stream to carry the vendor's reply", outcome.Body)
	}
}
