// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_chat_routes_test.go
// @for       Registration and boundary behavior of the Playground Chat route.
// @uses      internal/dataplane, internal/handler, internal/schema,
// internal/service, net/http, net/http/httptest, strings, testing.
// @reason    F4 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md requires
// the route itself to prove POST registration, wrong-verb rejection, and the
// data-plane credential boundary without a dashboard session.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-21
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

func newChatRouteRouter(t *testing.T) *Mux {
	t.Helper()
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Chat: handler.NewChatHandler(&service.ChatService{}),
	})
}

func TestChatRoute_BoundaryTable(t *testing.T) {
	mux := newChatRouteRouter(t)
	cases := []struct {
		name       string
		method     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "wrong verb", method: http.MethodGet, wantStatus: http.StatusMethodNotAllowed},
		{name: "no dashboard session", method: http.MethodPost, body: `{}`, wantStatus: http.StatusBadRequest, wantCode: dataplane.CodeValidation},
		{name: "malformed body remains data plane error", method: http.MethodPost, body: "not-json", wantStatus: http.StatusBadRequest, wantCode: dataplane.CodeValidation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, "/api/v1/chat/completions", strings.NewReader(tc.body))
			mux.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if tc.wantCode != "" && !strings.Contains(recorder.Body.String(), tc.wantCode) {
				t.Fatalf("body = %s, want data-plane code %s", recorder.Body.String(), tc.wantCode)
			}
		})
	}
}
