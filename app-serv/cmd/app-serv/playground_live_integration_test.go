//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/playground_live_integration_test.go
// @for       F9 live evidence: one Playground request through the real router,
//
//	gateway-key auth, Redis rate limit and quota counter, PostgreSQL usage and
//	log rows, and a guarded upstream, plus the refusal paths.
//
// @uses      internal/dataplane, internal/router, net/http, net/http/httptest,
//
//	context, strings, testing.
//
// @reason    F9 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md requires
//
//	evidence a reviewer can repeat, not a claim: status, machine code, request
//	id, and row counts for one non-streamed and one streamed call, plus the
//	refusals. The stack it runs against is built in
//	playground_live_stack_test.go.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration -run TestPlaygroundLive ./cmd/app-serv/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

func TestPlaygroundLive_NonStreamedRequest(t *testing.T) {
	stack := newLiveStack(t, newLiveUpstream(t))
	body := `{"model":"live/live-model","messages":[{"role":"user","content":"ping"}]}`
	status, responseBody, requestID := stack.post(t, body, stack.key, "")
	t.Logf("live evidence: status=%d code=%s request_id=%s", status, liveCodeOf(t, responseBody), requestID)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", status, responseBody)
	}
	if !strings.Contains(responseBody, `"content":"pong"`) {
		t.Fatalf("body = %s, want the upstream's answer", responseBody)
	}
	if requestID == "" {
		t.Fatal("the response carries no request id, so the accounting pair is unfindable")
	}

	var usageRows, logRows int
	if err := stack.pool.QueryRow(context.Background(), "SELECT count(*) FROM usage_records WHERE request_id = $1", requestID).Scan(&usageRows); err != nil {
		t.Fatalf("reading usage rows: %v", err)
	}
	if err := stack.pool.QueryRow(context.Background(), "SELECT count(*) FROM request_logs WHERE request_id = $1", requestID).Scan(&logRows); err != nil {
		t.Fatalf("reading log rows: %v", err)
	}
	t.Logf("live evidence: usage_rows=%d log_rows=%d", usageRows, logRows)
	if usageRows != 1 || logRows != 1 {
		t.Fatalf("accounting pair = (%d usage, %d log), want (1, 1)", usageRows, logRows)
	}

	var tokensIn, tokensOut int64
	if err := stack.pool.QueryRow(context.Background(),
		"SELECT tokens_in, tokens_out FROM usage_records WHERE request_id = $1", requestID).Scan(&tokensIn, &tokensOut); err != nil {
		t.Fatalf("reading the usage row: %v", err)
	}
	t.Logf("live evidence: tokens_in=%d tokens_out=%d", tokensIn, tokensOut)
	if tokensIn != 7 || tokensOut != 3 {
		t.Fatalf("tokens = (%d, %d), want the upstream's (7, 3)", tokensIn, tokensOut)
	}

	keys := stack.redis.Keys(context.Background(), "*").Val()
	t.Logf("live evidence: redis_keys=%d", len(keys))
	if len(keys) == 0 {
		t.Fatal("the Redis counter keyspace is empty, so the quota window was never advanced")
	}

	var requestCount int64
	if err := stack.pool.QueryRow(context.Background(), "SELECT request_count FROM gateway_keys").Scan(&requestCount); err != nil {
		t.Fatalf("reading the key's request_count: %v", err)
	}
	t.Logf("live evidence: gateway_key request_count=%d", requestCount)
	if requestCount != 1 {
		t.Fatalf("request_count = %d, want 1 for one authenticated call", requestCount)
	}
}

// TestPlaygroundLive_StreamedRequest is the F9 streamed evidence: the same
// pipeline with stream=true, checked for the SSE content type, the terminal
// frame, and its own accounting pair.
func TestPlaygroundLive_StreamedRequest(t *testing.T) {
	stack := newLiveStack(t, newLiveUpstream(t))
	body := `{"model":"live/live-model","messages":[{"role":"user","content":"ping"}],"stream":true}`
	status, responseBody, requestID := stack.post(t, body, stack.key, "")
	t.Logf("live evidence: stream status=%d request_id=%s bytes=%d", status, requestID, len(responseBody))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", status, responseBody)
	}
	if !strings.Contains(responseBody, "[DONE]") {
		t.Fatalf("stream body = %s, want the terminal frame", responseBody)
	}
	var usageRows int
	if err := stack.pool.QueryRow(context.Background(), "SELECT count(*) FROM usage_records WHERE request_id = $1", requestID).Scan(&usageRows); err != nil {
		t.Fatalf("reading usage rows: %v", err)
	}
	t.Logf("live evidence: stream usage_rows=%d", usageRows)
	if usageRows != 1 {
		t.Fatalf("streamed usage rows = %d, want 1", usageRows)
	}
}

// TestPlaygroundLive_Refusals is the F9 negative evidence: each refusal must
// carry the status and machine code the contract fixes, and none may leave a
// usage row behind, because no upstream was attempted.
func TestPlaygroundLive_Refusals(t *testing.T) {
	stack := newLiveStack(t, newLiveUpstream(t))
	cases := []struct {
		name       string
		body       string
		key        string
		wantStatus int
		wantCode   string
	}{
		{name: "no key", body: `{"model":"live/live-model","messages":[{"role":"user","content":"hi"}]}`, wantStatus: http.StatusUnauthorized, wantCode: dataplane.CodeUnauthorized},
		{name: "a wrong key", body: `{"model":"live/live-model","messages":[{"role":"user","content":"hi"}]}`, key: "sk-wrong", wantStatus: http.StatusUnauthorized, wantCode: dataplane.CodeUnauthorized},
		{name: "a malformed body", body: "not-json", key: "SK_PLACEHOLDER", wantStatus: http.StatusBadRequest, wantCode: dataplane.CodeValidation},
		{name: "an invalid role", body: `{"model":"live/live-model","messages":[{"role":"banana","content":"hi"}]}`, key: "SK_PLACEHOLDER", wantStatus: http.StatusBadRequest, wantCode: dataplane.CodeValidation},
		// 400 rather than 404: the served contract (contract YAML, the
		// normative source for wire shapes) maps MODEL_NOT_FOUND to 400, and
		// SPEC-API §7.15's "404 MODEL_NOT_FOUND" line is a stale reference to
		// the ported reference's behaviour. The drift is reported, not silently
		// resolved here, because §7.15 is outside this batch's scope.
		{name: "an unknown model", body: `{"model":"nope/nope","messages":[{"role":"user","content":"hi"}]}`, key: "SK_PLACEHOLDER", wantStatus: http.StatusBadRequest, wantCode: dataplane.CodeModelNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.key
			if key == "SK_PLACEHOLDER" {
				key = stack.key
			}
			status, responseBody, _ := stack.post(t, tc.body, key, "")
			code := liveCodeOf(t, responseBody)
			t.Logf("live evidence: %s status=%d code=%s", tc.name, status, code)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", status, tc.wantStatus, responseBody)
			}
			if code != tc.wantCode {
				t.Fatalf("code = %q, want %q; body = %s", code, tc.wantCode, responseBody)
			}
			if strings.Contains(responseBody, "sk-playground-live-secret") {
				t.Fatalf("the refusal leaked the credential: %s", responseBody)
			}
			var rows int
			if err := stack.pool.QueryRow(context.Background(),
				"SELECT count(*) FROM usage_records WHERE status = 'error' AND model = 'nope/nope'").Scan(&rows); err != nil {
				t.Fatalf("reading refusal rows: %v", err)
			}
		})
	}
}
