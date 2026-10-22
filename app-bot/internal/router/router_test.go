// Package router wires the app-bot HTTP surface.
//
// @file      internal/router/router_test.go
// @for       Route, request id, and metrics exposure contract tests.
// @uses      testing, net/http/httptest, logx, handler.
// @reason    The webhook and the metrics endpoint have different exposure rules, so the router is tested as a whole rather than handler by handler.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-10-21
package router_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

type ping struct{}

func (ping) Ping(context.Context) error { return nil }

type acceptAll struct{}

func (acceptAll) Accept(context.Context, schema.WorkItem) (bool, error) { return true, nil }

func newRouter() http.Handler {
	discard := logx.New("error", "test", io.Discard)
	return router.New(router.Deps{
		System:  handler.NewSystem(ping{}, ping{}, time.Second),
		Webhook: handler.NewWebhook("test-secret-1234567890", acceptAll{}, discard),
		Logger:  discard,
	})
}

func TestRoutes_GivenRequestVariations_ThenStatusAndHeadersAreCorrect(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		remoteAddr string
		headerKey  string
		headerVal  string
		wantStatus int
		wantHeader string
	}{
		{name: "health answers", method: http.MethodGet, path: "/healthz", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusOK},
		{name: "ready answers", method: http.MethodGet, path: "/readyz", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusOK},
		{name: "unknown route is not found", method: http.MethodGet, path: "/nope", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusNotFound},
		{name: "metrics from loopback is allowed", method: http.MethodGet, path: "/metrics", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusOK},
		{name: "metrics from a public address is refused", method: http.MethodGet, path: "/metrics", remoteAddr: "10.1.2.3:1234", wantStatus: http.StatusForbidden},
		{name: "request id is echoed", method: http.MethodGet, path: "/healthz", remoteAddr: "127.0.0.1:1234", headerKey: "X-Request-Id", headerVal: "req-12345678", wantStatus: http.StatusOK, wantHeader: "req-12345678"},
		{name: "request id is generated when absent", method: http.MethodGet, path: "/healthz", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusOK},
		{name: "webhook without a secret is refused", method: http.MethodPost, path: "/internal/telegram/webhook", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.headerKey != "" {
				req.Header.Set(tc.headerKey, tc.headerVal)
			}
			rec := httptest.NewRecorder()

			newRouter().ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			got := rec.Header().Get("X-Request-Id")
			if tc.wantHeader != "" && got != tc.wantHeader {
				t.Fatalf("X-Request-Id = %q, want %q", got, tc.wantHeader)
			}
			if tc.wantHeader == "" && got == "" {
				t.Fatalf("X-Request-Id is empty, want a generated value")
			}
			if tc.path == "/metrics" && tc.wantStatus == http.StatusForbidden {
				if strings.Contains(rec.Body.String(), "app_bot_up") {
					t.Fatalf("refused metrics still exposed the body: %s", rec.Body.String())
				}
			}
		})
	}
}
