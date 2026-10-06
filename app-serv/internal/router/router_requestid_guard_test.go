// Router serves the HTTP surface of app-serv.
//
// @file      internal/router/router_requestid_guard_test.go
// @for       The shape a caller-supplied request id must keep to be echoed back.
// @uses      net/http, net/http/httptest, strings, testing
// @reason    The middleware reflects this header into the response and writes it into every access-log line of the request, so an unbounded or control-character id is a megabyte of someone else's choosing in both, and a forged trace id that an operator then greps for.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-10-04
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes_RequestIDGuard(t *testing.T) {
	mux := newTestRouter(t)

	cases := []struct {
		name         string
		supplied     string
		wantEchoed   string
		wantReplaced bool
	}{
		{name: "a normal trace id is kept", supplied: "req-playground-1", wantEchoed: "req-playground-1"},
		{name: "an id at the cap is kept", supplied: strings.Repeat("a", maxRequestIDLength),
			wantEchoed: strings.Repeat("a", maxRequestIDLength)},
		{name: "one byte past the cap is replaced", supplied: strings.Repeat("a", maxRequestIDLength+1),
			wantReplaced: true},
		{name: "a control character is replaced", supplied: "ok\r\nX-Evil: yes", wantReplaced: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
			request.Header.Set(RequestIDHeader, tc.supplied)
			recorder := httptest.NewRecorder()

			mux.ServeHTTP(recorder, request)

			got := recorder.Header().Get(RequestIDHeader)
			if got == "" {
				t.Fatal("the response carries no request id at all")
			}
			if tc.wantReplaced {
				if got == tc.supplied {
					t.Fatalf("request id echoed the supplied %q, want a generated one", tc.supplied)
				}
				if len(got) > maxRequestIDLength {
					t.Fatalf("request id = %d bytes, want it bounded by the cap", len(got))
				}
				return
			}
			if got != tc.wantEchoed {
				t.Fatalf("request id = %q, want the caller's %q", got, tc.wantEchoed)
			}
		})
	}
}
