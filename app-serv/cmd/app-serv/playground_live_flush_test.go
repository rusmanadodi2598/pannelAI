//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/playground_live_flush_test.go
// @for       The F5 live evidence: a streamed answer through the real chain
//
//	must arrive frame by frame over a real socket, not as one blob.
//
// @uses      bufio, encoding/json, net, net/http, net/http/httptest, strconv,
// strings, testing, time.
//
// @reason    Draft 010 F5 is a wire-level defect, and httptest.ResponseRecorder
// implements Flush itself, so the only honest evidence is a real TCP
// connection through the production middleware chain. The upstream spaces
// its frames, so a gateway that buffers the whole answer cannot pass: the
// second frame simply does not exist yet when the first one is read.
//
//	PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	  go test -race -tags=integration -run TestPlaygroundLiveFlush ./cmd/app-serv/
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
	"strconv"
	"strings"
	"testing"
	"time"
)

// upstreamGap is how long the upstream holds its second frame back. It is wider
// than any reasonable scheduling delay and far narrower than a test timeout, so
// the arrival measurement is about the gateway's buffering and nothing else.
const upstreamGap = 750 * time.Millisecond

// spacedUpstream is a real SSE provider that separates its frames, so a gateway
// that writes the whole answer at the end cannot hide behind fast writes.
func spacedUpstream(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Stream bool `json:"stream"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !body.Stream {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-flush","object":"chat.completion","created":1,"model":"live-model","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	_, _ = w.Write([]byte("data: {\"id\":\"cmpl-flush\",\"object\":\"chat.completion.chunk\",\"model\":\"live-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"first\"},\"finish_reason\":null}]}\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
	time.Sleep(upstreamGap)
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
}

// TestPlaygroundLiveFlush_FirstFrameBeatsTheSecond is the F5 evidence: over a
// real socket, through the production chain, the first frame is delivered while
// the upstream is still holding the second one back.
func TestPlaygroundLiveFlush_FirstFrameBeatsTheSecond(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(spacedUpstream))
	t.Cleanup(upstream.Close)

	stack := newLiveStack(t, &liveUpstream{server: upstream})
	gateway := httptest.NewServer(stack.mux)
	t.Cleanup(gateway.Close)

	body := `{"model":"live/live-model","messages":[{"role":"user","content":"ping"}],"stream":true}`
	request := "POST /api/v1/chat/completions HTTP/1.1\r\n" +
		"Host: " + gateway.Listener.Addr().String() + "\r\n" +
		"Authorization: Bearer " + stack.key + "\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body

	conn, err := net.DialTimeout("tcp", gateway.Listener.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatalf("dialing the gateway: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("setting the socket deadline: %v", err)
	}
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("writing the request: %v", err)
	}

	reader := bufio.NewReader(conn)
	started := time.Now()
	line, err := reader.ReadString('\n')
	for err == nil && !strings.HasPrefix(line, "data: ") {
		line, err = reader.ReadString('\n')
	}
	if err != nil {
		t.Fatalf("reading the first data frame: %v", err)
	}
	if !strings.Contains(line, `"content":"first"`) {
		t.Fatalf("first frame = %q, want the upstream's first chunk", line)
	}
	elapsed := time.Since(started)
	t.Logf("live evidence: first frame after %s (upstream holds its second frame for %s)", elapsed, upstreamGap)
	if elapsed >= upstreamGap {
		t.Fatalf("the first frame took %s, which is not before the upstream's second frame (%s): the gateway is still buffering the answer", elapsed, upstreamGap)
	}

	terminal, err := reader.ReadString('\n')
	for err == nil && !strings.Contains(terminal, "[DONE]") {
		terminal, err = reader.ReadString('\n')
	}
	if err != nil {
		t.Fatalf("reading the terminal frame: %v", err)
	}
	t.Logf("live evidence: terminal frame arrived")
}
