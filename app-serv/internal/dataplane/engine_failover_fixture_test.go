// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_failover_fixture_test.go
// @for       The upstream doubles the failover tests drive: a per-model status
//
//	server and a per-credential one.
//
// @uses      encoding/json, net/http, net/http/httptest, strings, testing.
// @reason    A model-shaped refusal and a credential-shaped one are the two
//
//	questions the failover tests ask, and neither can be answered by the
//	relay fixture's fixed server. Keeping both here lets each test file
//	read as the rule it pins (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// probeUpstream answers every model with a complete completion, except the ones
// statusByModel marks, which answer their own status with an upstream-shaped
// error naming the model.
func probeUpstream(t *testing.T, calls *int, statusByModel map[string]int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		body := probeBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		if status, ok := statusByModel[body.Model]; ok && status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream refused ` + body.Model + `"}}`))
			return
		}
		writeProbeCompletion(w, body.Model)
	}))
	t.Cleanup(server.Close)
	return server
}

// credentialRefusingUpstream refuses every call made with one credential, with
// the given status, and serves the rest. A per-model status would answer a
// second account exactly as it answered the first, so only a credential-shaped
// refusal can prove the second account was tried because it is another account.
func credentialRefusingUpstream(t *testing.T, calls *int, refused string, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		body := probeBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.Header.Get("Authorization"), refused) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"this credential is refused"}}`))
			return
		}
		writeProbeCompletion(w, body.Model)
	}))
	t.Cleanup(server.Close)
	return server
}

// probeRequest is the shape an upstream double decodes from a call.
type probeRequest struct {
	Model string `json:"model"`
}

// probeBody decodes the model an upstream double received.
func probeBody(t *testing.T, r *http.Request) probeRequest {
	t.Helper()
	var body probeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Errorf("upstream received an undecodable body: %v", err)
	}
	return body
}

// writeProbeCompletion answers with one complete OpenAI completion carrying
// usage, so a served member is observable in the outcome.
func writeProbeCompletion(w http.ResponseWriter, model string) {
	_, _ = w.Write([]byte(`{"id":"chatcmpl-probe","object":"chat.completion","created":1,` +
		`"model":"` + model + `","choices":[{"index":0,"message":{"role":"assistant",` +
		`"content":"pong"},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
}
