// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/systemone_test.go
// @for       The decision route's rules: what it forwards, what it refuses, and
//
//	the accounting pair a call leaves.
//
// @uses      context, encoding/json, strings, testing, internal/dataplane,
//
//	internal/domain, internal/registry, internal/schema.
//
// @reason    SPEC-API-001 §7.15 serves the route and draft 029 F6 measured why it
//
//	matters: before it existed, `opencode/jev-1.13-free` resolved onto the
//	chat wire and a chat body was sent to a decision endpoint. These tests
//	pin the route's own contract (the body is forwarded verbatim with the
//	resolved model, the block's headers travel, the session is written) and
//	the refusals that keep the two planes from serving the same id.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// systemOneProvider is the provider a decision call resolves to: it declares the
// endpoint block and the identity headers the reference carries.
func systemOneProvider() registry.Provider {
	return registry.Provider{
		ID: "opencode", Display: registry.Display{Name: "OpenCode Free"},
		Category: "free", NoAuth: true, AuthType: registry.AuthNone,
		Transport: registry.Transport{BaseURL: "https://opencode.ai", Format: registry.DefaultFormat},
		SystemOne: &registry.SystemOneConfig{
			BaseURL: "https://opencode.ai/zen/v1/systemone",
			Headers: map[string]string{
				"x-opencode-client": "desktop",
				"User-Agent":        provider.OpenCodeUserAgent,
			},
		},
		Models: []registry.Model{{ID: "jev-1.13-free", Name: "Jev 1.13 Free", Kind: "systemone"}},
	}
}

// systemOneFixture builds the service over the collecting doubles.
func systemOneFixture(t *testing.T, entry registry.Provider, caller *stubMediaCaller, router MediaRouter) (*SystemOneService, *stubUsageRecorder, *stubLogRecorder) {
	t.Helper()
	// The kind comes from the entry's own declared model, so a case that changes
	// the entry really changes what the guard reads.
	kind := ""
	if len(entry.Models) > 0 {
		kind = entry.Models[0].Kind
	}
	usage, logs := &stubUsageRecorder{}, &stubLogRecorder{}
	svc, err := NewSystemOneService(SystemOneServiceDeps{
		Resolver: stubModelResolver{resolution: dataplane.Resolution{
			Provider: entry, ModelID: "jev-1.13-free", UpstreamID: "jev-1.13-free",
			Model: registry.Model{ID: "jev-1.13-free", Kind: kind},
		}},
		Router:    router,
		Caller:    caller,
		Usage:     usage,
		Logs:      logs,
		RequestID: func(context.Context) string { return "req_systemone" },
	})
	if err != nil {
		t.Fatalf("building the decision service: %v", err)
	}
	return svc, usage, logs
}

// decisionRequest builds the body a caller sends, exactly as the reference's own
// probe does (src/app/api/models/test/ping.js:138-146).
func decisionRequest(t *testing.T) schema.SystemOneRequest {
	t.Helper()
	raw := []byte(`{"model":"opencode/jev-1.13-free","state":"Customer: I was charged twice for my order this morning.",` +
		`"questions":{"probe":{"type":"noul","instructions":"Is the customer reporting a billing problem?"}}}`)
	req, err := schema.DecodeSystemOneRequest(raw)
	if err != nil {
		t.Fatalf("decoding the decision body: %v", err)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("validating the decision body: %v", err)
	}
	return req
}

// TestSystemOne_ForwardsTheDecisionPayloadVerbatim pins the route's contract: the
// body reaches the upstream with the model resolved onto its id, the entry's
// identity headers, and the session the Zen lanes require, and the answer comes
// back as the upstream wrote it.
func TestSystemOne_ForwardsTheDecisionPayloadVerbatim(t *testing.T) {
	answer := `{"model":"jev-1.13-free","answers":{"probe":{"type":"noul","noul":0.98}},"usage":{"input_tokens":286,"output_tokens":20},"cost":"0"}`
	caller := &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(answer)}}
	router := &stubMediaRouter{credential: provider.NoCredential("ep_opencode")}
	svc, usage, logs := systemOneFixture(t, systemOneProvider(), caller, router)

	got, err := svc.Decide(context.Background(), decisionRequest(t), "key-1")
	if err != nil {
		t.Fatalf("Decide() error = %v, want the decision answer", err)
	}
	if string(got) != answer {
		t.Fatalf("answer = %s, want the upstream's own bytes", got)
	}

	if len(caller.requests) != 1 {
		t.Fatalf("upstream saw %d calls, want 1", len(caller.requests))
	}
	sent := caller.requests[0]
	if sent.Method != "POST" {
		t.Fatalf("method = %q, want POST", sent.Method)
	}
	if sent.URL != "https://opencode.ai/zen/v1/systemone" {
		t.Fatalf("url = %q, want the entry's own decision endpoint", sent.URL)
	}
	// The block's own headers travel, and the session is written per call.
	if sent.Headers["x-opencode-client"] != "desktop" {
		t.Fatalf("x-opencode-client = %q, want desktop", sent.Headers["x-opencode-client"])
	}
	if !strings.HasPrefix(sent.Headers["User-Agent"], "opencode/") {
		t.Fatalf("User-Agent = %q, want the CLI identity", sent.Headers["User-Agent"])
	}
	if !provider.OpenCodeSessionRE.MatchString(sent.Headers["x-opencode-session"]) {
		t.Fatalf("x-opencode-session = %q, want the canonical shape", sent.Headers["x-opencode-session"])
	}
	// The credential is the lane's own: a keyless account sends none, which is
	// what makes the free decision model work without a stored key.
	if _, present := sent.Headers["Authorization"]; present {
		t.Fatalf("Authorization = %q, want none for a keyless account", sent.Headers["Authorization"])
	}

	// The payload carries the client's own members and the resolved model.
	var body map[string]json.RawMessage
	if err := json.Unmarshal(sent.Body, &body); err != nil {
		t.Fatalf("decoding the outbound body: %v", err)
	}
	if len(body) != 3 {
		t.Fatalf("outbound members = %d, want exactly model, state, questions", len(body))
	}
	if string(body["state"]) != `"Customer: I was charged twice for my order this morning."` {
		t.Fatalf("state = %s, want the client's own value", body["state"])
	}
	var questions map[string]json.RawMessage
	if err := json.Unmarshal(body["questions"], &questions); err != nil {
		t.Fatalf("decoding questions: %v", err)
	}
	if _, ok := questions["probe"]; !ok {
		t.Fatalf("questions = %v, want the client's own key", questions)
	}

	if router.successes != 1 {
		t.Fatalf("successes = %d, want the call recorded as served", router.successes)
	}
	if len(usage.rows) != 1 || len(logs.rows) != 1 {
		t.Fatalf("accounting = %d usage, %d logs, want one of each", len(usage.rows), len(logs.rows))
	}
}

// TestSystemOne_RefusesWhatItCannotServe pins the three refusals, each of which
// would otherwise spend an upstream call on a request that cannot be answered.
func TestSystemOne_RefusesWhatItCannotServe(t *testing.T) {
	chatProvider := systemOneProvider()
	chatProvider.SystemOne = nil

	noEndpoint := systemOneProvider()
	noEndpoint.SystemOne = &registry.SystemOneConfig{BaseURL: "  "}

	// The kind guard is not here: it lives in the resolver, so both planes read
	// one rule and neither can accept what the other refuses. Its cases are in
	// internal/dataplane/resolve_kind_test.go, against the real resolver.
	cases := []struct {
		name     string
		entry    registry.Provider
		wantCode string
	}{
		{name: "a provider without a decision block", entry: chatProvider, wantCode: dataplane.CodeProviderNotRoutable},
		{name: "a block with no base URL", entry: noEndpoint, wantCode: dataplane.CodeProviderNotRoutable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller := &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{}`)}}
			svc, _, logs := systemOneFixture(t, tc.entry, caller, &stubMediaRouter{})
			_, err := svc.Decide(context.Background(), decisionRequest(t), "key-1")
			if err == nil {
				t.Fatal("Decide() = nil error, want a refusal")
			}
			if code := dataplane.AsError(err).Code; code != tc.wantCode {
				t.Fatalf("code = %q, want %q", code, tc.wantCode)
			}
			if len(caller.requests) != 0 {
				t.Fatalf("upstream saw %d calls, want none", len(caller.requests))
			}
			// A refusal before the call leaves a log row and no usage row, the
			// shape register G20 fixed for the media plane.
			if len(logs.rows) != 1 {
				t.Fatalf("log rows = %d, want one for the refusal", len(logs.rows))
			}
		})
	}
}

// TestSystemOne_RecordsAnUpstreamRejection pins the failure direction: a non-2xx
// answer reaches the client as the upstream's own status and message, the key is
// reported failed, and the call is still recorded.
func TestSystemOne_RecordsAnUpstreamRejection(t *testing.T) {
	caller := &stubMediaCaller{answer: dataplane.MediaResponse{
		Status: 422, Body: []byte(`{"detail":"questions must not be empty"}`),
	}}
	router := &stubMediaRouter{}
	svc, usage, _ := systemOneFixture(t, systemOneProvider(), caller, router)

	_, err := svc.Decide(context.Background(), decisionRequest(t), "key-1")
	if err == nil {
		t.Fatal("Decide() = nil error, want the upstream rejection")
	}
	// A non-429 rejection answers as UPSTREAM_ERROR, which is the code the media
	// plane gives the same outcome.
	if code := dataplane.AsError(err).Code; code != dataplane.CodeUpstreamError {
		t.Fatalf("code = %q, want %q", code, dataplane.CodeUpstreamError)
	}
	if len(router.failures) != 1 {
		t.Fatalf("failures = %d, want the endpoint reported failed", len(router.failures))
	}
	if len(usage.rows) != 1 {
		t.Fatalf("usage rows = %d, want the call recorded even when rejected", len(usage.rows))
	}
}
