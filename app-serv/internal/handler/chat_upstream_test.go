// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_upstream_test.go
// @for       The upstream double the chat HTTP tests dial, and the model string
// that selects each of its behaviours.
// @uses      encoding/json, net/http, net/http/httptest, strings.
// @reason    F3 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md needs a
// failure before the first frame, a failure after one, and a malformed stream,
// which are three different upstream behaviours rather than three fixtures. The
// model string selects one, so the handler tests read as scenarios.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

// Upstream behaviours, selected by a substring of the requested model id. They
// are named for the failure each one produces on the client side.
const (
	upstreamPreFrame  = "preframe"
	upstreamPostFrame = "postframe"
	upstreamMalformed = "malformed"
)

type chatHTTPUpstream struct {
	server *httptest.Server
}

func (u *chatHTTPUpstream) serve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad upstream request", http.StatusBadRequest)
		return
	}
	switch {
	case strings.Contains(body.Model, upstreamPreFrame):
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream unavailable"}}`))
	case strings.Contains(body.Model, upstreamPostFrame):
		writeBrokenStream(w)
	case strings.Contains(body.Model, upstreamMalformed):
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {not json at all}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	case body.Stream:
		writeChunkedStream(w, "pong")
	default:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}
}

// writeChunkedStream sends one content chunk and the terminal marker, which is
// the smallest stream a client can act on.
func writeChunkedStream(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"" + content + "\"},\"finish_reason\":null}]}\n\n"))
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
}

// writeBrokenStream sends one complete frame and then drops the connection
// mid-line, which is what a provider dying after it started answering looks
// like: the client already has a committed stream and only a read error follows.
func writeBrokenStream(w http.ResponseWriter) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "the test upstream needs a hijackable connection", http.StatusInternalServerError)
		return
	}
	conn, buffered, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "the test upstream could not hijack", http.StatusInternalServerError)
		return
	}
	defer func() { _ = conn.Close() }()
	_, _ = buffered.WriteString("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nTransfer-Encoding: chunked\r\n\r\n")
	frame := "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"pong\"},\"finish_reason\":null}]}\n\n"
	_, _ = buffered.WriteString(strconv.FormatInt(int64(len(frame)), 16) + "\r\n" + frame + "\r\n")
	// A declared chunk that never arrives: the reader blocks on a body the
	// provider stopped sending, which is the failure the idle guard and the
	// read error both surface.
	_, _ = buffered.WriteString("40\r\n")
	_ = buffered.Flush()
}
