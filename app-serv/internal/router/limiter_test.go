// Package router tests configured request throttling behavior.
//
// @file      internal/router/limiter_test.go
// @for       Table-driven tests for the Redis rate-limiter middleware contract,
//
//	and the trusted-proxy rule that decides which address a request
//	is bucketed under.
//
// @uses      context, errors, net, net/http, net/http/httptest, testing, time,
//
//	internal/clientip, internal/repository.
//
// @reason    RATE_LIMIT_PER_MIN must produce generalized 429 envelopes and
//
//	preserve successful traffic at configured boundaries. R20 of
//	docs/DRAFT/042-CODE-REVIEW-FIXES.md added the address rule: a
//	request behind a proxy the operator named is bucketed by the
//	forwarded client, and a forgeable header is refused from any other
//	peer, so the limiter's bucket is pinned here against both.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-09-17
package router

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/clientip"
)

// recordingLimiter answers like the healthy store and records the client key
// each Allow carries, so a test can assert which address a request was
// bucketed under.
type recordingLimiter struct {
	keys []string
}

func (l *recordingLimiter) Allow(_ context.Context, clientKey string, _ int, _ time.Duration) (time.Duration, error) {
	l.keys = append(l.keys, clientKey)
	return 0, nil
}

type rateLimiterTestDouble struct {
	remaining time.Duration
	err       error
}

func (l rateLimiterTestDouble) Allow(context.Context, string, int, time.Duration) (time.Duration, error) {
	return l.remaining, l.err
}

func TestRequestRateLimitLeavesHealthUnmetered(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := requestRateLimit(next, rateLimiterTestDouble{remaining: time.Minute}, 1, nil)
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
			h := requestRateLimit(next, rateLimiterTestDouble{remaining: tc.remaining, err: tc.err}, tc.limit, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// TestRequestRateLimit_BucketsTheForwardedClientPastATrustedProxy pins the
// address rule the limiter buckets by: a request through a proxy the operator
// named is counted against the forwarded client, a forged header from any
// other peer is refused, and with no trusted proxies configured the header is
// never read at all.
func TestRequestRateLimit_BucketsTheForwardedClientPastATrustedProxy(t *testing.T) {
	trusted, err := clientip.ParseTrusted([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("ParseTrusted() error = %v", err)
	}

	cases := []struct {
		name    string
		peer    string
		chain   string
		trusted []*net.IPNet
		wantKey string
	}{
		{
			name:    "a trusted proxy's chain names the forwarded client",
			peer:    "10.0.0.9:41230",
			chain:   "203.0.113.7, 10.0.0.9",
			trusted: trusted,
			wantKey: "203.0.113.7",
		},
		{
			name:    "an untrusted peer cannot choose its own bucket",
			peer:    "203.0.113.5:44301",
			chain:   "9.9.9.9",
			trusted: trusted,
			wantKey: "203.0.113.5",
		},
		{
			name:    "no trusted proxies keeps the peer",
			peer:    "203.0.113.5:44301",
			chain:   "9.9.9.9",
			trusted: nil,
			wantKey: "203.0.113.5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			limiter := &recordingLimiter{}
			h := requestRateLimit(next, limiter, 1, tc.trusted)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
			request.RemoteAddr = tc.peer
			request.Header.Set("X-Forwarded-For", tc.chain)
			h.ServeHTTP(httptest.NewRecorder(), request)
			if len(limiter.keys) != 1 {
				t.Fatalf("Allow was called %d times, want once", len(limiter.keys))
			}
			if limiter.keys[0] != tc.wantKey {
				t.Fatalf("bucket key = %q, want %q", limiter.keys[0], tc.wantKey)
			}
		})
	}
}
