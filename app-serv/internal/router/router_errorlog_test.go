// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_errorlog_test.go
// @for       The §1.6/§8 rule that a failed request names its error code on the
//
//	access log line (register G9).
//
// @uses      bytes, log/slog, net/http, net/http/httptest, strings, testing.
// @reason    The access line already carried the status and the request id; what
//
//	it lacked was the "why", so a media 502 or an OAuth callback 500
//	named nothing an operator could act on. The code travels from the
//	envelope writers to the logging recorder through the writer chain, so
//	these tests drive the real mux rather than the recorder alone — the
//	envelope middleware's forwarding is exactly what they pin — and they
//	pin the negative case too: a served request must carry no code field.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureAccessLog points the default logger at a buffer for one test, so the
// access line the middleware writes can be read back. Router tests run
// sequentially, so the swap cannot race another test's logging.
func captureAccessLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buffer := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buffer, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return buffer
}

// accessLine returns the last access log line the middleware wrote.
func accessLine(t *testing.T, buffer *bytes.Buffer) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		t.Fatalf("no access log line captured: %q", buffer.String())
	}
	return lines[len(lines)-1]
}

// TestRoutes_AccessLogNamesTheFailureCode pins register G9: a failed request's
// access line carries the machine code beside the request id, whichever envelope
// wrote it — a handler's management error, the mux's own 404/405, or a data-plane
// error. The media-wired fixture is the one router that carries both a
// management and a data-plane route, so one table can cover all three writers.
func TestRoutes_AccessLogNamesTheFailureCode(t *testing.T) {
	cases := []struct {
		name     string
		method   string
		path     string
		body     string
		wantCode string
	}{
		{"handler error", http.MethodGet, "/api/v1/gateway-keys/gky_missing", "", "NOT_FOUND"},
		{"unknown route", http.MethodGet, "/api/v1/nope", "", "NOT_FOUND"},
		{"verb mismatch", http.MethodDelete, "/api/v1/gateway-keys", "", "METHOD_NOT_ALLOWED"},
		{"data plane refusal", http.MethodPost, "/api/v1/images/generations", `{"model":"openai/gpt-image-1"}`, "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer := captureAccessLog(t)
			mux := newManagementRouter(t)
			cookie := loginCookie(t, mux)

			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)

			line := accessLine(t, buffer)
			if !strings.Contains(line, "code="+tc.wantCode) {
				t.Fatalf("access line = %q, want code=%s", line, tc.wantCode)
			}
			if !strings.Contains(line, "request_id=") || !strings.Contains(line, "status=") {
				t.Fatalf("access line = %q, want the request id and status beside the code", line)
			}
		})
	}
}

// TestRoutes_ServedRequestLogsNoCode pins the negative side: the code field is
// absent for a request that succeeded, so the field's presence means exactly
// "this request failed".
func TestRoutes_ServedRequestLogsNoCode(t *testing.T) {
	buffer := captureAccessLog(t)
	mux := newTestRouter(t)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/version", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", recorder.Code, recorder.Body.String())
	}
	if line := accessLine(t, buffer); strings.Contains(line, "code=") {
		t.Fatalf("access line = %q, want no code field for a served request", line)
	}
}
