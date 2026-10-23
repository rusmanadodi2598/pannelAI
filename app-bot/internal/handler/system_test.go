// Package handler holds the HTTP surface of app-bot.
//
// @file      internal/handler/system_test.go
// @for       Health, readiness, and metrics contract tests.
// @uses      testing, net/http/httptest, errors.
// @reason    Readiness must fail closed when a dependency is down, so each dependency failure has its own case rather than a single happy path.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-21
package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/handler"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error { return f(ctx) }

func okPing(context.Context) error { return nil }

func failPing(context.Context) error { return errors.New("dependency is down") }

func TestSystemRoutes_GivenDependencyState_ThenStatusReflectsReadiness(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		postgres   pingerFunc
		redis      pingerFunc
		wantStatus int
		wantBody   string
	}{
		{name: "health is always ok", path: "/healthz", postgres: okPing, redis: okPing, wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "ready when both dependencies answer", path: "/readyz", postgres: okPing, redis: okPing, wantStatus: http.StatusOK, wantBody: "ready"},
		{name: "not ready when postgres fails", path: "/readyz", postgres: failPing, redis: okPing, wantStatus: http.StatusServiceUnavailable, wantBody: "postgres"},
		{name: "not ready when redis fails", path: "/readyz", postgres: okPing, redis: failPing, wantStatus: http.StatusServiceUnavailable, wantBody: "redis"},
		{name: "metrics answers text", path: "/metrics", postgres: okPing, redis: okPing, wantStatus: http.StatusOK, wantBody: "app_bot_up 1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			system := handler.NewSystem(tc.postgres, tc.redis, time.Second)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			switch tc.path {
			case "/healthz":
				system.Health(rec, req)
			case "/readyz":
				system.Ready(rec, req)
			case "/metrics":
				system.Metrics(rec, req)
			}

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if !contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body = %q, want it to contain %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestReady_GivenFailure_ThenNoInternalDetailLeaks(t *testing.T) {
	system := handler.NewSystem(pingerFunc(failPing), pingerFunc(okPing), time.Second)
	rec := httptest.NewRecorder()
	system.Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var payload struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if contains(rec.Body.String(), "dependency is down") {
		t.Fatalf("body leaked the internal error: %s", rec.Body.String())
	}
	if payload.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", payload.Status)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
