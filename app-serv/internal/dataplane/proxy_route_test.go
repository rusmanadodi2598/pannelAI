// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_route_test.go
// @for       Tests for the pool-driven dial: failover across the plan's
//
//	attempts, the bounded walk, and the connect-stage classification.
//
// @uses      internal/domain, context, errors, net, net/http, net/url/httptest,
//
//	crypto/x509, strings, testing.
//
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D7: the walk is what makes the
//
//	pool serve traffic instead of decorating it, so its rules are pinned
//	here against httptest doubles: a dead candidate is a dial to a closed
//	port, a live one is a server that answers, and the request body must
//	survive the walk because the first attempt consumes it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package dataplane

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// fakePlanner answers each Plan call with the next plan in the slice and
// records what the dialer reports.
type fakePlanner struct {
	plans    [][]domain.ProxyRouteAttempt
	planErr  error
	seen     []string
	reported []string
}

func (f *fakePlanner) Plan(_ context.Context, host string) ([]domain.ProxyRouteAttempt, error) {
	f.seen = append(f.seen, host)
	if f.planErr != nil {
		return nil, f.planErr
	}
	if len(f.plans) == 0 {
		return nil, nil
	}
	plan := f.plans[0]
	f.plans = f.plans[1:]
	return plan, nil
}

func (f *fakePlanner) ReportFailure(_ context.Context, proxyID string) {
	f.reported = append(f.reported, proxyID)
}

func attemptFor(id, raw string) domain.ProxyRouteAttempt {
	parsed, _ := url.Parse(raw)
	return domain.ProxyRouteAttempt{ID: id, URL: parsed}
}

func TestProxyDialer_WithoutRoutes(t *testing.T) {
	// A nil interface, not a nil *fakePlanner: a typed nil would make the
	// routes field non-nil and the dialer would call Plan on it.
	var planner ProxyRoutePlanner
	dialer := &ProxyDialer{client: http.DefaultClient, routes: planner}
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://127.0.0.1:1/x", strings.NewReader("body"))
	response, err := dialer.Do(context.Background(), request)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("Do() reached a closed port without failing")
	}
}

func TestProxyDialer_EmptyPlanUsesTheSharedClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "shared")
	}))
	defer server.Close()

	planner := &fakePlanner{}
	dialer := &ProxyDialer{client: server.Client(), routes: planner}
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/x", strings.NewReader("body"))

	response, err := dialer.Do(context.Background(), request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = response.Body.Close()
	if len(planner.seen) != 1 {
		t.Fatalf("Plan was consulted %d times, want once", len(planner.seen))
	}
}

func TestProxyDialer_FailsOverOnAConnectFailure(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		_, _ = io.WriteString(w, "served")
	}))
	defer server.Close()

	planner := &fakePlanner{plans: [][]domain.ProxyRouteAttempt{
		{attemptFor("prx_dead", "http://127.0.0.1:1"), attemptFor("prx_live", server.URL)},
	}}
	dialer := &ProxyDialer{client: server.Client(), routes: planner}
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://destination.example/x", strings.NewReader("the-body"))

	response, err := dialer.Do(context.Background(), request)
	if err != nil {
		t.Fatalf("Do() error = %v, want the live attempt to serve", err)
	}
	raw, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(raw) != "served" {
		t.Fatalf("the response body = %q, want the live attempt's answer", raw)
	}
	if len(bodies) != 1 || bodies[0] != "the-body" {
		t.Fatalf("the live attempt read body %v, want the payload once", bodies)
	}
	if len(planner.reported) != 1 || planner.reported[0] != "prx_dead" {
		t.Fatalf("reported failures = %v, want [prx_dead]", planner.reported)
	}
}

func TestProxyDialer_WalkIsBounded(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = io.WriteString(w, "served")
	}))
	defer server.Close()

	plan := []domain.ProxyRouteAttempt{attemptFor("prx_1", "http://127.0.0.1:1"), attemptFor("prx_2", "http://127.0.0.1:1"), attemptFor("prx_3", "http://127.0.0.1:1"), attemptFor("prx_4", server.URL)}
	planner := &fakePlanner{plans: [][]domain.ProxyRouteAttempt{plan}}
	dialer := &ProxyDialer{client: server.Client(), routes: planner}
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://destination.example/x", strings.NewReader("body"))

	response, err := dialer.Do(context.Background(), request)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("Do() succeeded past the attempt bound")
	}
	if hits != 0 {
		t.Fatalf("the live candidate was dialed %d times, want it left unreached by the bound", hits)
	}
	if len(planner.reported) != domain.MaxProxyRouteAttempts {
		t.Fatalf("reported failures = %d, want one per walked attempt", len(planner.reported))
	}
}

func TestProxyDialer_PlanFailureRefusesTheRequest(t *testing.T) {
	planner := &fakePlanner{planErr: errors.New("settings read failed")}
	dialer := &ProxyDialer{client: http.DefaultClient, routes: planner}
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://destination.example/x", strings.NewReader("body"))

	response, err := dialer.Do(context.Background(), request)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("Do() accepted a request whose plan could not be read")
	}
}

func TestIsProxyConnectFailure(t *testing.T) {
	opErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "a dial error through the proxy",
			err:  &url.Error{Op: "Post", URL: "http://proxy", Err: opErr},
			want: true,
		},
		{
			name: "a timeout at the proxy",
			err:  &url.Error{Op: "Post", URL: "http://proxy", Err: &timeoutNetError{}},
			want: true,
		},
		{
			name: "a TLS failure the destination answered for",
			err:  &url.Error{Op: "Post", URL: "http://proxy", Err: x509.UnknownAuthorityError{}},
			want: false,
		},
		{
			name: "a plain error carries no dial evidence",
			err:  errors.New("something else"),
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isProxyConnectFailure(tc.err); got != tc.want {
				t.Fatalf("isProxyConnectFailure(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "i/o timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }
