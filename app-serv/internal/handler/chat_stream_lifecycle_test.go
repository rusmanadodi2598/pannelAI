// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_stream_lifecycle_test.go
// @for       The stream status lifecycle: when the response is committed, and
// what a failure before or after that point looks like to a client.
// @uses      internal/schema, net/http, net/http/httptest, strings, testing.
// @reason    F3 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md found the
// handler committing 200 and the SSE headers before the relay ran, so a failure
// with no frame yet answered a success status carrying a JSON error body.
// OWASP A10 requires a failed operation not to report success, and AGENTS.md
// §1.3 requires the status to represent the error, so the commit point is
// pinned here rather than left to review.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestChatCompletionsHTTP_StreamStatusLifecycle pins the commit rule from both
// sides: a failure with no frame is an ordinary HTTP error, and a successful
// stream is committed as SSE and terminated.
func TestChatCompletionsHTTP_StreamStatusLifecycle(t *testing.T) {
	fixture := newChatHTTPFixture(t)
	handler := NewChatHandler(fixture.chat)
	cases := []struct {
		name        string
		model       string
		wantStatus  int
		wantType    string
		wantBody    string
		rejectBody  string
		wantNoFrame bool
	}{
		{
			name: "pre-frame failure is an HTTP error", model: "test/" + upstreamPreFrame,
			wantStatus: http.StatusBadGateway, wantType: "application/json", wantBody: "UPSTREAM_ERROR",
			wantNoFrame: true,
		},
		{
			name: "successful stream is SSE and terminal", model: "test/model",
			wantStatus: http.StatusOK, wantType: "text/event-stream", wantBody: "[DONE]",
		},
		{
			name: "post-frame failure ends the stream without a terminal frame", model: "test/" + upstreamPostFrame,
			wantStatus: http.StatusOK, wantType: "text/event-stream", wantBody: "pong",
			rejectBody: "[DONE]",
		},
		{
			name: "malformed upstream payload still terminates", model: "test/" + upstreamMalformed,
			wantStatus: http.StatusOK, wantType: "text/event-stream", wantBody: "[DONE]",
		},
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
			if tc.rejectBody != "" && strings.Contains(recorder.Body.String(), tc.rejectBody) {
				t.Fatalf("the committed stream carried a second error body: %s", recorder.Body.String())
			}
			if tc.wantNoFrame && strings.Contains(recorder.Header().Get("Content-Type"), "text/event-stream") {
				t.Fatalf("a pre-frame failure answered with SSE headers: %v", recorder.Header())
			}
		})
	}
}

// TestChatCompletionsHTTP_StreamHeadersCarryTheSpecValues pins the §4 headers a
// stream must carry: a buffering proxy in front of the gateway would otherwise
// turn a live answer into one delayed blob.
func TestChatCompletionsHTTP_StreamHeadersCarryTheSpecValues(t *testing.T) {
	fixture := newChatHTTPFixture(t)
	handler := NewChatHandler(fixture.chat)
	body := strings.TrimSuffix(chatBody("test/model"), "}") + `,"stream":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	recorder := httptest.NewRecorder()
	handler.Completions(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	want := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"X-Accel-Buffering": "no",
	}
	for header, value := range want {
		if got := recorder.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

// TestSSESink_CommitPoint pins the sink rule directly, which is the mechanism
// the two tests above observe through HTTP: the status is written exactly once,
// on the first frame, and never on a sink that received none.
func TestSSESink_CommitPoint(t *testing.T) {
	cases := []struct {
		name        string
		frames      []string
		wantWrote   bool
		wantStatus  int
		wantNoWrite bool
	}{
		{name: "no frame", wantWrote: false, wantStatus: http.StatusOK, wantNoWrite: true},
		{name: "one frame", frames: []string{"data: {}\n\n"}, wantWrote: true, wantStatus: http.StatusOK},
		{name: "several frames", frames: []string{"data: {}\n\n", "data: [DONE]\n\n"}, wantWrote: true, wantStatus: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			sink := newSSESink(recorder)
			for _, frame := range tc.frames {
				if err := sink.WriteFrame([]byte(frame)); err != nil {
					t.Fatalf("WriteFrame() error = %v", err)
				}
			}
			if sink.wrote() != tc.wantWrote {
				t.Fatalf("wrote() = %v, want %v", sink.wrote(), tc.wantWrote)
			}
			if tc.wantNoWrite && recorder.Code == http.StatusOK && recorder.Body.Len() == 0 && !recorder.Flushed {
				// A recorder that received no write reports 200 by default; the
				// property under test is that no header was set at all.
				if len(recorder.Header()) != 0 {
					t.Fatalf("an uncommitted sink set headers: %v", recorder.Header())
				}
				return
			}
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			if recorder.Header().Get("Content-Type") != "text/event-stream" {
				t.Fatalf("content type = %q, want text/event-stream", recorder.Header().Get("Content-Type"))
			}
		})
	}
}

// TestSSESink_WriteFailureIsReturned pins that a write failure reaches the
// caller rather than being swallowed, because the relay stops the stream on it.
func TestSSESink_WriteFailureIsReturned(t *testing.T) {
	sink := newSSESink(failingResponseWriter{})
	if err := sink.WriteFrame([]byte("data: {}\n\n")); err == nil {
		t.Fatal("WriteFrame() = nil error, want the write failure")
	}
	if !sink.wrote() {
		t.Fatal("a committed frame was not recorded as written")
	}
}

// failingResponseWriter fails every write, which is what a client that hung up
// mid-stream looks like to the sink.
type failingResponseWriter struct{ header http.Header }

func (w failingResponseWriter) Header() http.Header {
	if w.header == nil {
		return http.Header{}
	}
	return w.header
}
func (failingResponseWriter) Write([]byte) (int, error) { return 0, errors.New("client hung up") }
func (failingResponseWriter) WriteHeader(int)           {}
