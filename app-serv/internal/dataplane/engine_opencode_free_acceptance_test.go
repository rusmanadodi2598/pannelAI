// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_opencode_free_acceptance_test.go
// @for       The acceptance tests the owner asked for: the three OpenCode free
//
//	models answer with no credential, through the whole pipeline.
//
// @uses      context, encoding/json, testing, internal/schema.
// @reason    The fixtures live in engine_opencode_free_test.go so both files stay
//
//	inside the AGENTS.md section 1.1 budget. Keeping the scenarios separate
//	from the stand-in also makes the file read as what it asserts: the
//	upstream gate is measured once, and each test states one property of
//	the lane.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// openCodeFreeRequest builds the client request exactly as the chat handler does:
// the body decoded into the typed schema, so translation reads the same values a
// live caller would produce.
func openCodeFreeRequest(t *testing.T, model string) Request {
	t.Helper()
	raw := []byte(`{"model":"opencode/` + model + `","messages":[{"role":"user","content":"Say pong only."}],"stream":true}`)
	decoded, err := schema.DecodeChatRequest(raw)
	if err != nil {
		t.Fatalf("decoding the client body: %v", err)
	}
	return Request{
		Route: RouteChatCompletions, ClientFormat: schema.FormatOpenAI,
		Model: decoded.Model, Chat: &decoded, Raw: raw, Stream: decoded.Stream,
	}
}

// TestOpenCodeFree_ThreeModelsAnswerWithoutACredential is the acceptance test the
// owner asked for: each free model completes a streamed request through the whole
// pipeline with no key configured, and the upstream receives a request that
// satisfies the free-tier gate.
func TestOpenCodeFree_ThreeModelsAnswerWithoutACredential(t *testing.T) {
	seen := []openCodeFreeCall{}
	server := openCodeFreeUpstream(t, &seen)
	engine := newOpenCodeFreeEngine(t, server.URL, &seen)

	for _, model := range openCodeFreeModels {
		t.Run(model, func(t *testing.T) {
			sink := &recordingSink{}
			outcome, err := engine.Relay(context.Background(), openCodeFreeRequest(t, model), sink)
			if err != nil {
				t.Fatalf("Relay(%s) error = %v, want the free tier to answer", model, err)
			}
			if outcome.ProviderID != "opencode" {
				t.Fatalf("provider = %q, want opencode", outcome.ProviderID)
			}
			if outcome.EndpointID != "ep_free" {
				t.Fatalf("endpoint = %q, want the keyless one", outcome.EndpointID)
			}
			if !outcome.Streamed {
				t.Fatal("the answer must arrive as a stream: the free tier refuses a non-streamed request")
			}
			if text := freeSinkText(sink); text != "pong" {
				t.Fatalf("streamed text = %q, want pong", text)
			}
		})
	}

	if len(seen) != len(openCodeFreeModels) {
		t.Fatalf("upstream calls = %d, want %d", len(seen), len(openCodeFreeModels))
	}
	for index, call := range seen {
		model := openCodeFreeModels[index]
		if call.Model != model {
			t.Fatalf("call %d model = %q, want %q", index, call.Model, model)
		}
		if call.Authorization != "Bearer public" {
			t.Fatalf("%s: Authorization = %q, want the public bearer the connector writes", model, call.Authorization)
		}
		if call.Session == "" {
			t.Fatalf("%s: no canonical session was presented", model)
		}
		if !call.Stream {
			t.Fatalf("%s: the outbound body did not stream", model)
		}
		if model != openCodeFreeUngated {
			if missing := missingOpenCodeFreeTools(call.Tools); len(missing) > 0 {
				t.Fatalf("%s: the outbound body is missing %v", model, missing)
			}
		}
		if want := openCodeFreePath(model); call.Path != want {
			t.Fatalf("%s: path = %q, want %q", model, call.Path, want)
		}
	}
}

// TestOpenCodeFree_NoCredentialMaterialLeavesTheGateway pins the "without auth"
// half: the endpoint carries no key, the selection presents no material, and the
// only credential the upstream sees is the literal public bearer the connector
// writes. A regression that started requiring a stored key would fail here
// before it reached the acceptance test above.
func TestOpenCodeFree_NoCredentialMaterialLeavesTheGateway(t *testing.T) {
	seen := []openCodeFreeCall{}
	server := openCodeFreeUpstream(t, &seen)
	engine := newOpenCodeFreeEngine(t, server.URL, &seen)

	sink := &recordingSink{}
	if _, err := engine.Relay(context.Background(), openCodeFreeRequest(t, "space-bunny-free"), sink); err != nil {
		t.Fatalf("Relay() error = %v, want a keyless free-tier call to succeed", err)
	}
	if len(seen) != 1 {
		t.Fatalf("upstream calls = %d, want 1", len(seen))
	}
	// The endpoint stores no key at all, so a request that carried one would be
	// reading material that does not exist rather than a configured secret.
	if got := seen[0].Authorization; got != "Bearer public" {
		t.Fatalf("Authorization = %q, want only the public bearer", got)
	}
}

// TestOpenCodeFree_DeclaredResponsesModelTakesTheResponsesWire pins the per-model
// rule the port restored in draft 017: muse-spark-1.3-contributor-free is declared
// with target_format openai-responses, so a chat client's request must be
// translated and POSTed to /zen/v1/responses rather than forwarded to the chat
// path. The stand-in refuses the wrong path, so this fails loudly on a regression.
func TestOpenCodeFree_DeclaredResponsesModelTakesTheResponsesWire(t *testing.T) {
	seen := []openCodeFreeCall{}
	server := openCodeFreeUpstream(t, &seen)
	engine := newOpenCodeFreeEngine(t, server.URL, &seen)

	sink := &recordingSink{}
	outcome, err := engine.Relay(context.Background(),
		openCodeFreeRequest(t, "muse-spark-1.3-contributor-free"), sink)
	if err != nil {
		t.Fatalf("Relay() error = %v, want the declared Responses model to answer", err)
	}
	if len(seen) != 1 {
		t.Fatalf("upstream calls = %d, want 1", len(seen))
	}
	if seen[0].Path != "/zen/v1/responses" {
		t.Fatalf("path = %q, want the Responses leaf the model declares", seen[0].Path)
	}
	var body map[string]any
	if err := json.Unmarshal(seen[0].Body, &body); err != nil {
		t.Fatalf("the outbound body is not JSON: %v", err)
	}
	if _, present := body["input"]; !present {
		t.Fatalf("the outbound body has no input member: %s", seen[0].Body)
	}
	if _, present := body["messages"]; present {
		t.Fatalf("the outbound body still carries chat messages: %s", seen[0].Body)
	}
	if text := freeSinkText(sink); text != "pong" {
		t.Fatalf("streamed text = %q, want pong", text)
	}
	_ = outcome
}

// freeSinkText joins the text a sink received, so an assertion reads the answer
// rather than the frame framing. The engine writes client-format frames, so the
// extracted text is the same "pong" whichever wire the upstream answered on.
func freeSinkText(sink *recordingSink) string {
	var out strings.Builder
	for _, frame := range sink.frames {
		text := string(frame)
		if index := strings.Index(text, `"content":"`); index >= 0 {
			rest := text[index+len(`"content":"`):]
			if end := strings.Index(rest, `"`); end >= 0 {
				out.WriteString(rest[:end])
			}
			continue
		}
		if index := strings.Index(text, `"delta":"`); index >= 0 {
			rest := text[index+len(`"delta":"`):]
			if end := strings.Index(rest, `"`); end >= 0 {
				out.WriteString(rest[:end])
			}
		}
	}
	return out.String()
}

// TestOpenCodeFree_UnknownFreeIdStillPassesThrough pins the provider's declared
// passthrough: space-bunny-free and mimo-v2.6-flash-free are not in the static
// model list (the reference lists them only through its live model fetcher), so
// they must resolve as-is rather than being refused as unknown. This is the
// property that makes the owner's list servable without editing the registry.
func TestOpenCodeFree_UnknownFreeIdStillPassesThrough(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("loading the embedded registry: %v", err)
	}
	entry, _ := index.Provider("opencode")
	for _, model := range []string{"space-bunny-free", "mimo-v2.6-flash-free"} {
		if _, declared := index.Model("opencode", model); declared {
			t.Fatalf("%s is declared; this test pins the passthrough path it must use", model)
		}
		if !entry.PassthroughModels {
			t.Fatalf("opencode does not pass model ids through, so %s is unreachable", model)
		}
	}
}
