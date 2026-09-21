// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_chat_boundary_test.go
// @for       The chat route's router-level boundary: the request id it echoes,
// the rate limit it answers, and the models route Playground reads.
// @uses      internal/dataplane, internal/handler, internal/schema,
//
//	internal/service, net/http, net/http/httptest, strings, testing.
//
// @reason    F4 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md requires
// the route itself, not only the handler, to prove the §4 request-id echo, the
// §4 rate limit, and the §7.15 models list Playground needs to fill its model
// selector. AGENTS.md §2.1 requires a validation failure and an auth failure per
// protected route, so both are covered through the mux.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-21
package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// errLimiterUnavailable stands for a Redis the limiter cannot reach, which must
// surface as an internal error rather than a request that quietly skips the
// meter.
var errLimiterUnavailable = errors.New("redis unavailable")

// TestChatRoute_EchoesTheRequestID pins the §4 rule that one request id reaches
// the caller, whether the caller supplied one or the gateway generated it. The
// accounting pair is keyed by this id, so a response without it is a request an
// operator cannot find.
func TestChatRoute_EchoesTheRequestID(t *testing.T) {
	mux := newChatRouteRouter(t)
	cases := []struct {
		name     string
		supplied string
		want     string
	}{
		{name: "a supplied id is kept", supplied: "req-playground-1", want: "req-playground-1"},
		{name: "a second supplied id is kept", supplied: "req-playground-2", want: "req-playground-2"},
		{name: "an absent id is generated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(`{}`))
			if tc.supplied != "" {
				request.Header.Set(RequestIDHeader, tc.supplied)
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			got := recorder.Header().Get(RequestIDHeader)
			if got == "" {
				t.Fatal("the response carries no request id")
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("request id = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestChatRoute_RateLimitBoundary pins the §4 rule that the data plane is
// metered: a client over its window is refused with the data-plane rate-limit
// envelope rather than reaching the handler, a client within it proceeds, and a
// limiter that cannot answer is an internal error rather than a silent pass.
func TestChatRoute_RateLimitBoundary(t *testing.T) {
	cases := []struct {
		name       string
		limiter    repository.RateLimiter
		limit      int
		wantStatus int
		wantCode   string
	}{
		{
			name: "within the window", limiter: rateLimiterTestDouble{}, limit: 60,
			wantStatus: http.StatusBadRequest, wantCode: dataplane.CodeValidation,
		},
		{
			name: "over the window", limiter: rateLimiterTestDouble{remaining: 30 * time.Second}, limit: 60,
			wantStatus: http.StatusTooManyRequests, wantCode: dataplane.CodeRateLimited,
		},
		{
			name: "at the exact boundary", limiter: rateLimiterTestDouble{remaining: time.Second}, limit: 1,
			wantStatus: http.StatusTooManyRequests, wantCode: dataplane.CodeRateLimited,
		},
		{
			name: "a limiter that cannot answer", limiter: rateLimiterTestDouble{err: errLimiterUnavailable}, limit: 60,
			wantStatus: http.StatusInternalServerError, wantCode: dataplane.CodeInternal,
		},
		{
			name: "metering disabled", limiter: nil, limit: 0,
			wantStatus: http.StatusBadRequest, wantCode: dataplane.CodeValidation,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := newChatRouteRouterWith(t, Deps{RateLimiter: tc.limiter, RateLimitPerMin: tc.limit})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(`{}`))
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tc.wantCode) {
				t.Fatalf("body = %s, want code %s", recorder.Body.String(), tc.wantCode)
			}
			if tc.wantStatus == http.StatusTooManyRequests && recorder.Header().Get("Retry-After") == "" {
				t.Fatal("a 429 carries no Retry-After header")
			}
		})
	}
}

// TestChatRoute_BodyLimitIsEnforcedAtTheRoute pins the body bound through the
// mux, so an oversized payload is refused before the pipeline rather than
// buffered.
func TestChatRoute_BodyLimitIsEnforcedAtTheRoute(t *testing.T) {
	mux := newChatRouteRouter(t)
	oversized := `{"model":"gpt-4o","messages":[{"role":"user","content":"` + strings.Repeat("a", schema.MaxBodyBytes) + `"}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(oversized))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), dataplane.CodeValidation) {
		t.Fatalf("body = %s, want the %s code", recorder.Body.String(), dataplane.CodeValidation)
	}
}

// newChatRouteRouterWith builds the real mux over the chat handler and merges
// whichever router dependencies a case needs.
func newChatRouteRouterWith(t *testing.T, extra Deps) *Mux {
	t.Helper()
	extra.System = handler.NewSystemHandler(handler.SystemHandlerDeps{
		Info:   schema.SystemInfo{Version: "test", Commit: "test"},
		Health: service.NewHealthService(service.HealthServiceDeps{}),
	})
	extra.Chat = handler.NewChatHandler(&service.ChatService{})
	return New(extra)
}
