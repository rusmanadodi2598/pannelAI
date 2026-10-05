// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_stream_write_budget_test.go
// @for       The SSE sink's write budget: the deadline is renewed per frame, so
//
//	a stream that outlives the server's WriteTimeout still arrives.
//
// @uses      io, net/http, net/http/httptest, strings, testing, time.
// @reason    Go sets the server WriteTimeout once, absolutely, when the request
//
//	headers are read, so a stream running past it is cut on its next
//	frame with the client reading `unexpected EOF`. The sink must renew
//	the deadline, and the end-to-end form here is what caught the defect
//	(draft 042 R01).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-03
package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSSESink_RenewsTheWriteDeadline pins the rule that keeps a stream alive
// past the server's WriteTimeout: the sink renews the budget per frame.
func TestSSESink_RenewsTheWriteDeadline(t *testing.T) {
	writer := &deadlineResponseWriter{}
	sink := newSSESink(writer)
	for _, frame := range []string{"data: one\n\n", "data: two\n\n"} {
		if err := sink.WriteFrame([]byte(frame)); err != nil {
			t.Fatalf("WriteFrame() error = %v", err)
		}
	}
	if len(writer.deadlines) != 2 {
		t.Fatalf("SetWriteDeadline called %d times, want once per frame (2)", len(writer.deadlines))
	}
	for index, deadline := range writer.deadlines {
		if !deadline.After(time.Now()) {
			t.Errorf("deadline %d = %v, want a future time", index, deadline)
		}
	}
}

// TestSSESink_OutlivesServerWriteTimeout is the end-to-end form of the same
// rule, against a real server: with a WriteTimeout shorter than the gap between
// two frames, the second frame must still reach the client. Before the renewal
// existed this test failed with the client reading `unexpected EOF`.
func TestSSESink_OutlivesServerWriteTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		sink := newSSESink(w)
		if err := sink.WriteFrame([]byte("data: one\n\n")); err != nil {
			return
		}
		sink.Flush()
		time.Sleep(900 * time.Millisecond)
		if err := sink.WriteFrame([]byte("data: two\n\n")); err != nil {
			return
		}
		sink.Flush()
	})
	server := httptest.NewUnstartedServer(mux)
	server.Config.WriteTimeout = 300 * time.Millisecond
	server.Start()
	defer server.Close()

	response, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatalf("GET /stream: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), "data: two") {
		t.Fatalf("second frame missing after the WriteTimeout window; body = %q", body)
	}
}

// deadlineResponseWriter records the write deadlines the sink sets, which is
// the mechanism under test; the embedded recorder provides the rest of the
// ResponseWriter contract.
type deadlineResponseWriter struct {
	httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *deadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}
