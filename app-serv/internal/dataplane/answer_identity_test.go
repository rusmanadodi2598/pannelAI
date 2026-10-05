// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/answer_identity_test.go
// @for       The model name a client reads back when it addressed a combo.
//
// @uses      bytes, context, encoding/json, io, net/http, net/http/httptest,
//
//	strings, testing, internal/domain, internal/registry.
//
// @reason    A combo is a name an operator wrote in the panel and a client sends,
//
//	and a round_robin combo serves a different member on each request.
//	Answering with the member's id therefore tells the caller nothing it
//	can use, and nothing pinned that: the suite passed both before and after
//	the naming change. These tests hold the rule from three directions,
//	the folded answer, the re-framed stream, and the same-wire passthrough
//	that reaches no translation step at all, and hold plain model requests
//	where they were.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package dataplane

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// vendorLabel is the name an upstream calls its own answer. It is deliberately
// not a name the client knows: a combo member is reached through the combo, and a
// provider is free to answer under a label of its own.
const vendorLabel = "vendor-label-9000"

// newLabelingUpstream answers every model with a completion naming itself
// vendorLabel, streamed or not, so a test can tell the gateway's name for the
// answer apart from the upstream's.
func newLabelingUpstream(t *testing.T, calls *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		var body struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("upstream received an undecodable body: %v", err)
		}
		if body.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(`data: {"id":"chunk-1","model":"` + vendorLabel +
				`","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":"stop"}]` +
				sseDone))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-label","object":"chat.completion","created":1,` +
			`"model":"` + vendorLabel + `","choices":[{"index":0,"message":{"role":"assistant",` +
			`"content":"pong"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// labelingComboEngine wires one combo member on provider alpha to the labeling
// upstream, so a request aimed at the combo is served by a member whose own name
// is known to the test.
func labelingComboEngine(t *testing.T, combo domain.Combo) (*Engine, *memEndpointRepo) {
	t.Helper()
	var calls int
	server := newLabelingUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		combo.Name(): combo,
	})
	return engine, repo
}

func TestStampAnswerModel(t *testing.T) {
	const answer = `{"id":"chatcmpl-1","model":"auto","choices":[{"index":0}],"mystery":{"kept":true}}`
	cases := []struct {
		name  string
		model string
		raw   string
		want  string
	}{
		{name: "the label is replaced and every other member survives", model: "pi-agent", raw: answer,
			want: `"model":"pi-agent"`},
		{name: "an empty name leaves the bytes alone", model: "", raw: answer, want: `"model":"auto"`},
		{name: "the name it already carries is not rewritten", model: "auto", raw: answer, want: `"model":"auto"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(stampAnswerModel([]byte(tc.raw), tc.model))
			if !strings.Contains(got, tc.want) {
				t.Fatalf("stamped body must contain %s, got: %s", tc.want, got)
			}
			if !strings.Contains(got, `"mystery":{"kept":true}`) {
				t.Fatalf("a field the schema does not model was dropped: %s", got)
			}
		})
	}
}

func TestStampAnswerModel_LeavesABodyItCannotRead(t *testing.T) {
	raw := []byte(`not json at all`)
	if got := stampAnswerModel(raw, "pi-agent"); !bytes.Equal(got, raw) {
		t.Fatalf("an undecodable body was rewritten: %s", got)
	}
}

func TestFoldChatStreamNamesTheCombo(t *testing.T) {
	body := "data: {\"id\":\"c1\",\"model\":\"" + vendorLabel +
		"\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"pong\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	upstream := &Upstream{Body: io.NopCloser(strings.NewReader(body))}
	resolution := Resolution{
		Provider: registry.Provider{ID: "alpha"},
		ModelID:  "works",
		Target:   TargetOpenAI,
		Combo:    comboRow("pi-agent", "alpha/works"),
	}

	folded, _, err := foldStream(upstream, resolution, nil)
	if err != nil {
		t.Fatalf("foldStream() error = %v", err)
	}
	if !strings.Contains(string(folded), `"model":"pi-agent"`) {
		t.Fatalf("a folded combo answer must name the combo, got: %s", folded)
	}
	if strings.Contains(string(folded), vendorLabel) {
		t.Fatalf("the upstream's label reached the caller: %s", folded)
	}
}

// TestRelay_ComboNamesTheComboItWasAddressedAt is the reported bug: three combos,
// each a name the operator wrote, answered every request with the id of whichever
// member happened to be up. The client cannot send that id back, and it is not the
// model it asked for.
func TestRelay_ComboNamesTheComboItWasAddressedAt(t *testing.T) {
	engine, _ := labelingComboEngine(t, comboRow("pi-agent", "alpha/works"))

	outcome, err := engine.Relay(context.Background(), relayRequest("pi-agent"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	body := string(outcome.Body)
	if !strings.Contains(body, `"model":"pi-agent"`) {
		t.Fatalf("a combo answer must name the combo, got: %s", body)
	}
	if strings.Contains(body, vendorLabel) {
		t.Fatalf("the upstream's label reached the caller: %s", body)
	}
	// The routing identity stays the member's: the usage row and the quota window
	// are billed against what was actually called, and Outcome.Combo already
	// carries the combo for the panel to group by.
	if outcome.Model != "works" {
		t.Fatalf("Outcome.Model = %q, want the served member so billing names what ran", outcome.Model)
	}
	if outcome.Combo != "pi-agent" {
		t.Fatalf("Outcome.Combo = %q, want pi-agent", outcome.Combo)
	}
	if outcome.VisionAdapted {
		t.Fatal("Outcome.VisionAdapted = true, want false: no adapter served this answer")
	}
}

func TestRelay_ComboStreamNamesTheComboInEveryFrame(t *testing.T) {
	engine, _ := labelingComboEngine(t, comboRow("pc-agent", "alpha/works"))
	in := relayRequest("pc-agent")
	in.Stream = true
	sink := &recordingSink{}

	if _, err := engine.Relay(context.Background(), in, sink); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if len(sink.frames) == 0 {
		t.Fatal("the sink received no frames")
	}
	for _, frame := range sink.frames {
		text := string(frame)
		if !strings.Contains(text, sseDoneMarker) && !strings.Contains(text, `"model":"pc-agent"`) {
			t.Fatalf("a frame named something other than the combo: %s", text)
		}
		if strings.Contains(text, vendorLabel) {
			t.Fatalf("the upstream's label reached the caller: %s", text)
		}
	}
}

// TestRelay_PlainModelNamesTheAddressedModel guards the other half of the rule: a
// request that addressed no combo still gets back the name it sent. The vendor's
// label is not one, it answers `alpha/works` as a bare or aliased id the gateway
// will not route, so a caller that read it back and re-sent it got MODEL_NOT_FOUND.
func TestRelay_PlainModelNamesTheAddressedModel(t *testing.T) {
	var calls int
	server := newLabelingUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{})

	outcome, err := engine.Relay(context.Background(), relayRequest("alpha/works"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if !strings.Contains(string(outcome.Body), `"model":"alpha/works"`) {
		t.Fatalf("a plain request was not served the name it addressed, got: %s", outcome.Body)
	}
	if strings.Contains(string(outcome.Body), vendorLabel) {
		t.Fatalf("the upstream's label reached the caller: %s", outcome.Body)
	}
	// Billing still names what actually ran, which is the resolved member rather
	// than the address the caller used.
	if outcome.Model != "works" {
		t.Fatalf("Outcome.Model = %q, want the served member so billing names what ran", outcome.Model)
	}
}

// sseDoneMarker is the terminal marker, which carries no model field to check.
const sseDoneMarker = "[DONE]"
