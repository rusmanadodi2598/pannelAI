// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_refusal_reading_test.go
// @for       Tests for how the walk reads an upstream refusal, and for the trace it leaves of the address that served.
// @uses      internal/domain, context, log/slog, net/http, net/http/httptest, strings, testing, time.
// @reason    The refusal reading is a two-way decision (which status belongs to the address, which wait that address should sit out) and the trace is the only evidence an operator gets that rotation and failover happened at all, so both are pinned apart from the walk mechanics in proxy_address_failover_test.go (AGENTS.md §1.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-15
package dataplane

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestProxyDialer_RecordsTheAddressThatServed pins the walk's only trace: which
// candidate carried the request, and which ones it had to step over. Spread is the
// pool's purpose, and an operator weighing a rotation change has no evidence
// otherwise.
func TestProxyDialer_RecordsTheAddressThatServed(t *testing.T) {
	dialer, _, _ := refusedThenServed(t, http.StatusTooManyRequests, nil)
	var logged strings.Builder
	dialer.logger = slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	response, err := postToDialer(t, dialer)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = response.Body.Close()

	out := logged.String()
	for _, want := range []string{"prx_refused", "prx_served", "skipped_candidates=1", "status=200", "destination.example"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the walk recorded %q, missing %q", out, want)
		}
	}
	if !strings.Contains(out, "refused an address before serving") {
		t.Fatalf("a walk that stepped over an address must be readable as one: %q", out)
	}
}

// TestProxyDialer_RecordsACleanServeQuietly pins that the ordinary case is not the
// one that raises its voice: no skip, no warning.
func TestProxyDialer_RecordsACleanServeQuietly(t *testing.T) {
	served := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "served")
	}))
	defer served.Close()

	planner := &fakePlanner{plans: [][]domain.ProxyRouteAttempt{{attemptFor("prx_only", served.URL)}}}
	var logged strings.Builder
	dialer := &ProxyDialer{client: served.Client(), routes: planner,
		logger: slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://destination.example/x", strings.NewReader("the-body"))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}

	response, err := dialer.Do(context.Background(), request, "opencode")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = response.Body.Close()

	out := logged.String()
	if !strings.Contains(out, "level=DEBUG") || !strings.Contains(out, "skipped_candidates=0") {
		t.Fatalf("a clean serve must log at debug with nothing skipped, got %q", out)
	}
}

func TestClassifyAddressRefusal(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		header  map[string]string
		wantGo  bool
		wantTTL time.Duration
	}{
		{name: "the vendor says this address asked too often", status: http.StatusTooManyRequests, wantGo: true, wantTTL: domain.ProxyFailureCooldown},
		{name: "a quota exhaustion the vendor states as a payment refusal", status: http.StatusPaymentRequired, wantGo: false, wantTTL: 0},
		{name: "the free tier gate that is not about capacity", status: http.StatusForbidden, wantGo: false, wantTTL: 0},
		{name: "a server error is the upstream's own problem", status: http.StatusInternalServerError, wantGo: false, wantTTL: 0},
		{name: "a served answer carries no refusal", status: http.StatusOK, wantGo: false, wantTTL: 0},
		{name: "a stated wait is honored", status: http.StatusTooManyRequests,
			header: map[string]string{"Retry-After": "45"}, wantGo: true, wantTTL: 45 * time.Second},
		{name: "a wait past the ceiling is clamped", status: http.StatusTooManyRequests,
			header: map[string]string{"Retry-After": "7200"}, wantGo: true, wantTTL: domain.ProxyFailureCooldown},
		{name: "an unparsable wait falls back to the cooldown", status: http.StatusTooManyRequests,
			header: map[string]string{"Retry-After": "soon"}, wantGo: true, wantTTL: domain.ProxyFailureCooldown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := &http.Response{StatusCode: tc.status, Header: http.Header{}}
			for name, value := range tc.header {
				response.Header.Set(name, value)
			}
			got := classifyAddressRefusal(response)
			if got.failover != tc.wantGo {
				t.Fatalf("failover = %v, want %v", got.failover, tc.wantGo)
			}
			if got.parkFor != tc.wantTTL {
				t.Fatalf("parkFor = %v, want %v", got.parkFor, tc.wantTTL)
			}
		})
	}
}
