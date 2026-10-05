// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_http_test.go
// @for       HTTP behavior coverage for the Playground Chat completions route.
// @uses      internal/dataplane, internal/handler, net/http, net/http/httptest,
// strings, testing.
// @reason    F4 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md requires
// the happy, validation, authentication, stream, and pre-frame failure paths
// through the handler rather than only through helper or engine tests.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-21
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatCompletionsHTTP_Table(t *testing.T) {
	fixture := newChatHTTPFixture(t)
	handler := NewChatHandler(fixture.chat)
	cases := []struct {
		name       string
		key        string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "happy path", key: fixture.key, body: chatBody("test/model"), wantStatus: http.StatusOK},
		{name: "malformed JSON", key: fixture.key, body: "not-json", wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR"},
		{name: "unknown role", key: fixture.key, body: `{"model":"test/model","messages":[{"role":"banana","content":"hello"}]}`, wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR"},
		{name: "missing key", body: chatBody("test/model"), wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "invalid key", key: "sk-wrong", body: chatBody("test/model"), wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		// 404: §7.15 ends the resolution order here and the OpenAI wire answers an unknown
		// model with it, so a client's retry policy reads the truth.
		{name: "unknown model", key: fixture.key, body: chatBody("ghost/not-a-model"), wantStatus: http.StatusNotFound, wantCode: "MODEL_NOT_FOUND"},
		{name: "upstream pre-frame failure", key: fixture.key, body: chatBody("test/preframe"), wantStatus: http.StatusBadGateway, wantCode: "UPSTREAM_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(tc.body))
			if tc.key != "" {
				request.Header.Set("Authorization", "Bearer "+tc.key)
			}
			recorder := httptest.NewRecorder()
			handler.Completions(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if tc.wantCode != "" && !strings.Contains(recorder.Body.String(), tc.wantCode) {
				t.Fatalf("body = %s, want code %s", recorder.Body.String(), tc.wantCode)
			}
		})
	}
}

func TestChatCompletionsHTTP_StreamLifecycle(t *testing.T) {
	fixture := newChatHTTPFixture(t)
	handler := NewChatHandler(fixture.chat)
	cases := []struct {
		name       string
		model      string
		wantStatus int
		wantType   string
		wantBody   string
	}{
		{name: "successful stream", model: "test/model", wantStatus: http.StatusOK, wantType: "text/event-stream", wantBody: "[DONE]"},
		{name: "pre-frame failure remains HTTP error", model: "test/preframe", wantStatus: http.StatusBadGateway, wantType: "application/json", wantBody: "UPSTREAM_ERROR"},
		// A stream for a model that resolves to nothing fails before any frame, so it is
		// still an ordinary HTTP answer: the status the non-streamed path gives, not a 200
		// under an SSE type, and not a 400 that invites the same retry.
		{name: "unknown model before the first frame", model: "ghost/not-a-model", wantStatus: http.StatusNotFound, wantType: "application/json", wantBody: "MODEL_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.TrimSuffix(chatBody(tc.model), "}") + `,"stream":true}`
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer "+fixture.key)
			recorder := httptest.NewRecorder()
			handler.Completions(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if !strings.Contains(recorder.Header().Get("Content-Type"), tc.wantType) {
				t.Fatalf("content type = %q, want %q", recorder.Header().Get("Content-Type"), tc.wantType)
			}
			if !strings.Contains(recorder.Body.String(), tc.wantBody) {
				t.Fatalf("body = %s, want %q", recorder.Body.String(), tc.wantBody)
			}
		})
	}
}

func chatBody(model string) string {
	return `{"model":"` + model + `","messages":[{"role":"user","content":"hello"}]}`
}
