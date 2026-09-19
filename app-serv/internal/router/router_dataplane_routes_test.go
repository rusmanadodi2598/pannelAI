// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_dataplane_routes_test.go
// @for       Route-table tests for the §7.15 Responses data-plane route.
// @uses      internal/dataplane, internal/handler, internal/schema,
//
//	internal/service, net/http, net/http/httptest, strings, testing.
//
// @reason    AGENTS.md §2.1 requires a validation-failure case per route, and
//
//	§7.15's Responses route is the one a coding agent calls, so a
//	forgotten registration would look like an outage rather than a 404.
//	The service is deliberately unwired: every case below fails in the
//	route's own decode and validation, which is what the route owns.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
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

// newResponsesRouteRouter builds the real mux with the data-plane handler wired
// over an unwired service. No case below reaches the service, so the engine is
// never needed and the route's own contract is what is under test.
func newResponsesRouteRouter(t *testing.T) *Mux {
	t.Helper()
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Chat: handler.NewChatHandler(&service.ChatService{}),
	})
}

// TestResponsesRoute_ValidationFailure pins the §7.15 Responses route: it is
// registered under POST, and a body that breaks its contract is refused with the
// §8 envelope rather than reaching the pipeline.
func TestResponsesRoute_ValidationFailure(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "a body that is not JSON", body: "not json"},
		{name: "a body with no model", body: `{"input":"hi"}`},
		{name: "a model longer than the contract allows", body: `{"model":"` + strings.Repeat("m", 201) + `","input":"hi"}`},
		{name: "a ceiling below one", body: `{"model":"gpt-4o","input":"hi","max_output_tokens":0}`},
		{name: "a ceiling that is negative", body: `{"model":"gpt-4o","input":"hi","max_output_tokens":-1}`},
		{name: "an input that is neither a string nor an array", body: `{"model":"gpt-4o","input":42}`},
	}
	mux := newResponsesRouteRouter(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/responses", strings.NewReader(tc.body)))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), dataplane.CodeValidation) {
				t.Fatalf("body = %s, want the %s code", recorder.Body.String(), dataplane.CodeValidation)
			}
		})
	}
}

// TestResponsesRoute_IsNotSessionGated pins the property the data-plane table
// exists for: a CLI tool cannot hold a dashboard cookie, so the route answers
// without one. A session-gated route would answer 401 here.
func TestResponsesRoute_IsNotSessionGated(t *testing.T) {
	mux := newResponsesRouteRouter(t)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/responses", strings.NewReader(`{}`)))
	if recorder.Code == http.StatusUnauthorized {
		t.Fatalf("route answered 401 without a session cookie: %s", recorder.Body.String())
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", recorder.Code, recorder.Body.String())
	}
}

// TestResponsesRoute_WrongVerbIsRefused pins that the route is registered for the
// verb the spec fixes, so a client that mistypes it gets the mux's 405 rather
// than a request that silently reaches a handler.
func TestResponsesRoute_WrongVerbIsRefused(t *testing.T) {
	mux := newResponsesRouteRouter(t)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/responses", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}
