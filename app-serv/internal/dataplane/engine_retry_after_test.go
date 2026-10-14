// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_retry_after_test.go
// @for       Tests for the vendor's Retry-After on the routing path: the parse, the bound the client is told, and the wait that parks the credential.
// @uses      internal/domain, net/http, errors, testing, time.
// @reason    The header was already parsed for the transport's own same-target retry but never reached the park, so a 429 naming a two-minute wait was answered with a two-second one and the next request walked straight back into the limiter. The same number is also owed to the client, and both bounds have to be pinned: a vendor could park a key for a day with one header line.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

func TestFailureRetryAfter(t *testing.T) {
	cases := []struct {
		name   string
		header http.Header
		err    error
		want   time.Duration
	}{
		{name: "a seconds form is read", err: &UpstreamError{Status: http.StatusTooManyRequests, Message: "slow down",
			Header: http.Header{"Retry-After": []string{"120"}}}, want: 2 * time.Minute},
		{name: "an absent header names nothing", err: &UpstreamError{Status: http.StatusTooManyRequests,
			Header: http.Header{}}, want: 0},
		{name: "a failure with no upstream answer names nothing", err: errors.New("dial tcp: no route to host"), want: 0},
		{name: "a zero wait is not a hint", err: &UpstreamError{Status: http.StatusTooManyRequests,
			Header: http.Header{"Retry-After": []string{"0"}}}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := failureRetryAfter(tc.err); got != tc.want {
				t.Fatalf("failureRetryAfter() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestClampRetrySeconds pins the client-facing bound to the park's own ceiling, so
// one number cannot mean two things in one gateway.
func TestClampRetrySeconds(t *testing.T) {
	ceiling := int(domain.RateLimitCeiling / time.Second)
	cases := []struct {
		name string
		wait time.Duration
		want int
	}{
		{name: "a two minute wait is handed over whole", wait: 2 * time.Minute, want: 120},
		{name: "one second is one second", wait: time.Second, want: 1},
		{name: "a day is held to the park's ceiling", wait: 24 * time.Hour, want: ceiling},
		{name: "a sub-second wait is not rounded up", wait: 400 * time.Millisecond, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampRetrySeconds(tc.wait); got != tc.want {
				t.Fatalf("clampRetrySeconds(%v) = %d, want %d", tc.wait, got, tc.want)
			}
		})
	}
	if ceiling != 300 {
		t.Fatalf("Retry-After ceiling = %d seconds, want the rate-limit ladder's 5m", ceiling)
	}
}

// TestTranslateCallErrorCarriesRetryAfter pins that the dead handler write is alive:
// a 429 with a wait reaches the client as Retry-After, and one without names nothing.
func TestTranslateCallErrorCarriesRetryAfter(t *testing.T) {
	engine := &Engine{}
	throttled := &UpstreamError{Status: http.StatusTooManyRequests, Message: "slow down",
		Header: http.Header{"Retry-After": []string{"45"}}}

	failure := AsError(engine.translateCallError(throttled))
	if failure.Code != CodeRateLimited {
		t.Fatalf("code = %s, want %s", failure.Code, CodeRateLimited)
	}
	if failure.RetryAfter != 45 {
		t.Fatalf("Retry-After = %d, want the 45 seconds the vendor named", failure.RetryAfter)
	}

	plain := &UpstreamError{Status: http.StatusTooManyRequests, Message: "slow down"}
	if got := AsError(engine.translateCallError(plain)).RetryAfter; got != 0 {
		t.Fatalf("Retry-After = %d, want no header for a vendor that named no wait", got)
	}

	beyond := &UpstreamError{Status: http.StatusTooManyRequests, Message: "gone",
		Header: http.Header{"Retry-After": []string{"86400"}}}
	if got := AsError(engine.translateCallError(beyond)).RetryAfter; got != 300 {
		t.Fatalf("Retry-After = %d, want the ceiling: a vendor cannot park a key past its own ladder", got)
	}
}

// TestProviderRetryAfterParsesBothForms pins the shared parser the transport and the
// router now read from one implementation.
func TestProviderRetryAfterParsesBothForms(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "seconds", value: "30", want: true},
		{name: "empty names nothing", value: ""},
		{name: "a negative is not a wait", value: "-5"},
		{name: "junk is not a wait", value: "soon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{"Retry-After": []string{tc.value}}
			got := provider.RetryAfter(header)
			if (got > 0) != tc.want {
				t.Fatalf("RetryAfter(%q) = %v, want positive = %v", tc.value, got, tc.want)
			}
			if tc.want && got != 30*time.Second {
				t.Fatalf("RetryAfter(%q) = %v, want 30s", tc.value, got)
			}
		})
	}
}
