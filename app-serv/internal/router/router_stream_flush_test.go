// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_stream_flush_test.go
// @for       The streaming boundary: every wrapper in the production chain must
// forward http.Flusher, so a frame reaches the client while the handler runs.
// @uses      bufio, net/http, net/http/httptest, sync, testing, time.
// @reason    F5 of docs/DRAFT/010-USAGE-ENDPOINT-READINESS.md found the chain
// hiding http.Flusher from the SSE sink: Go promotes only the methods of the
// embedded interface, so `w.(http.Flusher)` failed inside the chain, the sink
// stored a nil flusher, and every stream arrived as one blob when the handler
// returned. The rule is pinned over a real socket and at every layer, because
// httptest.ResponseRecorder implements Flush itself and reports the broken
// chain as healthy.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-22
package router

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// flushSpy is the innermost writer. It counts the flushes that reach it, which
// is what "the frame reached the client" reduces to below the socket.
type flushSpy struct {
	header  http.Header
	flushes int
}

func (w *flushSpy) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *flushSpy) Write(b []byte) (int, error) { return len(b), nil }
func (w *flushSpy) WriteHeader(int)             {}
func (w *flushSpy) Flush()                      { w.flushes++ }

// TestStreamFlush_EveryLayerForwardsFlusher walks the layers the production
// chain stacks, with a bare handler as the control that must stay untouched:
// each one has to keep the capability its inner writer offers.
func TestStreamFlush_EveryLayerForwardsFlusher(t *testing.T) {
	cases := []struct {
		name string
		wrap func(http.Handler) http.Handler
	}{
		{name: "bare handler (control)", wrap: func(next http.Handler) http.Handler { return next }},
		{name: "logging response recorder", wrap: logging},
		{name: "envelope status recorder", wrap: envelope},
		{name: "production chain", wrap: chain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &flushSpy{}
			seen := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				flusher, ok := w.(http.Flusher)
				if !ok {
					return
				}
				seen = true
				flusher.Flush()
			})
			tc.wrap(handler).ServeHTTP(spy, httptest.NewRequest(http.MethodGet, APIVersion+"/chat/completions", nil))
			if !seen {
				t.Fatal("the writer the handler received does not implement http.Flusher")
			}
			if spy.flushes == 0 {
				t.Fatal("Flush() never reached the writer below the chain")
			}
		})
	}
}

// TestStreamFlush_ControllerReachesThroughUnwrap pins the second half of the
// same rule for http.ResponseController, which unwraps rather than asserts: a
// wrapper that forwards Flush but not Unwrap still hides every other optional
// capability from a caller that asks for it by controller.
func TestStreamFlush_ControllerReachesThroughUnwrap(t *testing.T) {
	cases := []struct {
		name string
		wrap func(http.Handler) http.Handler
	}{
		{name: "bare handler (control)", wrap: func(next http.Handler) http.Handler { return next }},
		{name: "production chain", wrap: chain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &flushSpy{}
			var controllerErr error
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				controllerErr = http.NewResponseController(w).Flush()
			})
			tc.wrap(handler).ServeHTTP(spy, httptest.NewRequest(http.MethodGet, APIVersion+"/chat/completions", nil))
			if controllerErr != nil {
				t.Fatalf("ResponseController.Flush() error = %v", controllerErr)
			}
			if spy.flushes == 0 {
				t.Fatal("ResponseController.Flush() never reached the writer below the chain")
			}
		})
	}
}

// TestStreamFlush_FirstFrameArrivesBeforeHandlerReturns is the permanent form
// of the audit's probe: a real socket, the production chain, and a handler that
// stays open. The connection is dialed by hand rather than through the client,
// because the HTTP client returns only after the response headers arrive, and
// the defect this test pins is precisely that nothing arrives. The first line
// has to be read while the handler is still holding the connection, which is
// what a client watching a live answer does.
func TestStreamFlush_FirstFrameArrivesBeforeHandlerReturns(t *testing.T) {
	const (
		firstLine = "data: first\n\n"
		lastLine  = "data: [DONE]\n"
		deadline  = 3 * time.Second
	)
	cases := []struct {
		name string
		wrap func(http.Handler) http.Handler
	}{
		{name: "bare handler (control)", wrap: func(next http.Handler) http.Handler { return next }},
		{name: "production chain", wrap: chain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			release := make(chan struct{})
			var once sync.Once
			releaseHandler := func() { once.Do(func() { close(release) }) }
			defer releaseHandler()

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(firstLine))
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				<-release
				_, _ = w.Write([]byte(lastLine))
			})
			server := httptest.NewServer(tc.wrap(handler))
			t.Cleanup(server.Close)

			conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), deadline)
			if err != nil {
				t.Fatalf("dialing %s: %v", server.URL, err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			if err := conn.SetDeadline(time.Now().Add(deadline)); err != nil {
				t.Fatalf("setting the socket deadline: %v", err)
			}
			// HTTP/1.0 keeps the body unframed: no chunked encoding, so a line read
			// is a line the gateway wrote, which is what makes the arrival timing
			// measurable rather than an artifact of transfer framing.
			if _, err := conn.Write([]byte("GET / HTTP/1.0\r\nHost: test\r\n\r\n")); err != nil {
				t.Fatalf("writing the request line: %v", err)
			}

			reader := bufio.NewReader(conn)
			// The status line itself is the first arrival the socket can observe:
			// the handler committed it with the first frame, so it must be readable
			// while the handler is still holding the connection open.
			statusLine, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("reading the status line while the handler is open: %v", err)
			}
			if fields := strings.Fields(statusLine); len(fields) < 2 || fields[1] != "200" {
				t.Fatalf("status line = %q, want a 200", statusLine)
			}
			for {
				headerLine, err := reader.ReadString('\n')
				if err != nil {
					t.Fatalf("reading the headers: %v", err)
				}
				if headerLine == "\r\n" || headerLine == "\n" {
					break
				}
			}
			first, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("reading the first frame while the handler is open: %v", err)
			}
			if first != "data: first\n" {
				t.Fatalf("first line = %q, want %q", first, "data: first\n")
			}

			releaseHandler()
			// ReadString returns one line, so the frame's blank terminator is a
			// second read: two reads cover the first frame's tail and the terminal
			// frame's own first line, which is what the handler wrote after release.
			blank, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("reading the first frame's terminator: %v", err)
			}
			if blank != "\n" {
				t.Fatalf("the first frame's terminator = %q, want a blank line", blank)
			}
			terminal, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("reading the terminal frame after the handler released: %v", err)
			}
			if terminal != "data: [DONE]\n" {
				t.Fatalf("terminal line = %q, want %q", terminal, "data: [DONE]\n")
			}
		})
	}
}
