// Package router tests configured request throttling behavior.
//
// @file      internal/router/limiter_test.go
// @for       Table-driven tests for the Redis rate-limiter middleware contract.
// @uses      context, net/http/httptest, testing, time, internal/repository.
// @reason    RATE_LIMIT_PER_MIN must produce generalized 429 envelopes and
//
//	preserve successful traffic at configured boundaries.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-09-17
package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type rateLimiterTestDouble struct {
	remaining time.Duration
	err       error
}

func (l rateLimiterTestDouble) Allow(context.Context, string, int, time.Duration) (time.Duration, error) {
	return l.remaining, l.err
}

func TestRequestRateLimitLeavesHealthUnmetered(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := requestRateLimit(next, rateLimiterTestDouble{remaining: time.Minute}, 1)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("health status=%d want=%d", w.Code, http.StatusNoContent)
	}
}

func TestRequestRateLimitTable(t *testing.T) {
	cases := []struct {
		name      string
		remaining time.Duration
		limit     int
		err       error
		want      int
	}{
		{name: "allowed request", remaining: 0, limit: 1, want: http.StatusNoContent},
		{name: "boundary rejection", remaining: time.Second, limit: 1, want: http.StatusTooManyRequests},
		{name: "disabled limiter", remaining: time.Minute, limit: 0, want: http.StatusNoContent},
		{name: "backend failure", remaining: 0, limit: 10, err: errors.New("redis unavailable"), want: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			h := requestRateLimit(next, rateLimiterTestDouble{remaining: tc.remaining, err: tc.err}, tc.limit)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
