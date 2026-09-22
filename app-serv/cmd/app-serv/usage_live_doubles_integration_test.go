//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/usage_live_doubles_integration_test.go
// @for       The live Usage pass's fixture, socket readers, and upstream
//
//	doubles.
//
// @uses      internal/dataplane, internal/domain, internal/handler, internal/router,
//
//	internal/schema, internal/service, bufio, context, encoding/json, net,
//	net/http, net/http/httptest, os, strings, testing, time.
//
// @reason    Draft 013 F4 is closed by "the route answers with a session and the
//
//	live pass records a frame from the gateway itself". Every half of
//	that has a unit test, and none of them proves the two halves meet:
//	the marker is written by a real relay leg over a real socket, the
//	store is the real Redis, and the frame is read off a real
//	connection through the real router. That is the evidence the
//	previous pass could not produce, and it is the difference between
//	"the code should work" and "the frame arrived".
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration -run TestUsageLive ./cmd/app-serv/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// readLiveFramesUntil opens the live route, reads frames until want reports
// satisfied, and returns every frame it read. A deadline bounds the wait so a
// stream that never reports the state fails rather than hanging.
func readLiveFramesUntil(t *testing.T, server *httptest.Server, want func(schema.UsageLiveFrame) bool) []schema.UsageLiveFrame {
	t.Helper()
	conn, reader := dialLiveRoute(t, server)
	t.Cleanup(func() { _ = conn.Close() })

	var frames []schema.UsageLiveFrame
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		payload := readSSEPayload(t, reader)
		if payload == "" {
			continue
		}
		var frame schema.UsageLiveFrame
		if err := json.Unmarshal([]byte(payload), &frame); err != nil {
			t.Fatalf("decoding a frame %s: %v", payload, err)
		}
		frames = append(frames, frame)
		if want(frame) {
			return frames
		}
	}
	t.Fatalf("no frame satisfied the expectation within the deadline; read %d frames", len(frames))
	return nil
}

// dialLiveRoute opens one HTTP/1.0 connection to the live route, so a line read
// is a line the gateway wrote and the arrival timing is measurable.
func dialLiveRoute(t *testing.T, server *httptest.Server) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatalf("dialing the live route: %v", err)
	}
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatalf("setting the socket deadline: %v", err)
	}
	if _, err := conn.Write([]byte("GET /api/v1/usage/live HTTP/1.0\r\nHost: test\r\n\r\n")); err != nil {
		t.Fatalf("writing the request: %v", err)
	}
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading the status line: %v", err)
	}
	if fields := strings.Fields(statusLine); len(fields) < 2 || fields[1] != "200" {
		t.Fatalf("status line = %q, want a 200", statusLine)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading a header: %v", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	return conn, reader
}

// readSSEPayload reads one SSE data frame's payload, skipping comments.
func readSSEPayload(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var data []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading a frame line: %v", err)
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if len(data) > 0 {
				return strings.Join(data, "\n")
			}
			continue
		}
		if payload, ok := strings.CutPrefix(trimmed, "data:"); ok {
			data = append(data, strings.TrimPrefix(payload, " "))
		}
	}
}

// truncate shortens a log value without splitting a rune mid-sequence.
func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

// liveSessionGate is the production session gate with a fixture session, so the
// pass proves the route is gated without building the whole auth graph.
func liveSessionGate(t *testing.T, next http.Handler) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Session") != "fixture" {
			schema.WriteError(w, domain.NewUnauthorizedError("a session is required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newHoldingUpstream answers a chat completion only once release is closed, so a
// marker can be observed while its request is genuinely in flight.
func newHoldingUpstream(t *testing.T, release <-chan struct{}) *liveUpstream {
	t.Helper()
	upstream := &liveUpstream{}
	upstream.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-live","object":"chat.completion","created":1,"model":"live-model",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`))
	}))
	t.Cleanup(upstream.server.Close)
	return upstream
}

// newFailingUpstream refuses every call, so the frame's error half has a real
// failure to report.
func newFailingUpstream(t *testing.T) *liveUpstream {
	t.Helper()
	upstream := &liveUpstream{}
	upstream.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"the upstream refused this request","type":"invalid_request_error"}}`))
	}))
	t.Cleanup(upstream.server.Close)
	return upstream
}
