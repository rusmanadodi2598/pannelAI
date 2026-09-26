// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_connect_rejection_test.go
// @for       Tests for the CONNECT-rejection class: a proxy that answers the
//
//	tunnel request with a non-200 status must park and fail over, not
//	pass the answer off as the upstream's failure.
//
// @uses      internal/domain, context, errors, io, net, net/http,
//
//	net/http/httptest, net/url, strings, testing.
//
// @reason    The live failover test on 2026-09-26 found this class open: Go
//
//	reports a non-200 CONNECT as a plain error, so the walk read it as
//	the upstream's and refused to spend the next candidate. The doubles
//	here are a proxy that refuses the tunnel and one that opens it, so
//	the class is pinned end to end over a real TLS destination.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package dataplane

import (
	"context"
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

// refusingProxy answers every CONNECT with 407, the class a proxy whose
// credentials or plan do not cover this endpoint produces.
func refusingProxy() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "this double only answers CONNECT", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Proxy-Authenticate", `Basic realm=""`)
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
}

// tunnelingProxy opens the CONNECT tunnel to its target, which is what a
// healthy pool candidate does for an HTTPS destination.
func tunnelingProxy() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "this double only tunnels CONNECT", http.StatusMethodNotAllowed)
			return
		}
		upstream, err := net.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			_ = upstream.Close()
			http.Error(w, "the double cannot hijack", http.StatusInternalServerError)
			return
		}
		client, _, err := hijacker.Hijack()
		if err != nil {
			_ = upstream.Close()
			return
		}
		_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
		go func() {
			_, _ = io.Copy(upstream, client)
			_ = upstream.Close()
		}()
		_, _ = io.Copy(client, upstream)
		_ = client.Close()
	}))
}

// TestProxyDialer_FailsOverWhenTheProxyRefusesTheConnect pins the class the
// live test found: a candidate whose CONNECT is answered with 407 never
// delivered the request, so the walk must report it and let the next candidate
// serve, exactly as it does for a dial failure.
func TestProxyDialer_FailsOverWhenTheProxyRefusesTheConnect(t *testing.T) {
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "served")
	}))
	defer destination.Close()
	refusing := refusingProxy()
	defer refusing.Close()
	tunneling := tunnelingProxy()
	defer tunneling.Close()

	planner := &fakePlanner{plans: [][]domain.ProxyRouteAttempt{
		{attemptFor("prx_refusing", refusing.URL), attemptFor("prx_tunneling", tunneling.URL)},
	}}
	dialer := &ProxyDialer{client: destination.Client(), routes: planner}
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, destination.URL+"/x", strings.NewReader("the-body"))

	response, err := dialer.Do(context.Background(), request, "")
	if err != nil {
		t.Fatalf("Do() error = %v, want the tunnel candidate to serve", err)
	}
	raw, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(raw) != "served" {
		t.Fatalf("the response body = %q, want the tunnel candidate's answer", raw)
	}
	if len(planner.reported) != 1 || planner.reported[0] != "prx_refusing" {
		t.Fatalf("reported failures = %v, want [prx_refusing]", planner.reported)
	}
}

// TestIsProxyConnectFailure_ConnectRejection pins the classifier's new case:
// the typed marker the transport hook returns, and not the message text a
// proxy could echo back inside an unrelated failure.
func TestIsProxyConnectFailure_ConnectRejection(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "the typed rejection the hook returns",
			err: &url.Error{
				Op: "Post", URL: "https://destination",
				Err: &proxyConnectRejected{status: http.StatusProxyAuthRequired},
			},
			want: true,
		},
		{
			name: "the same words without the type",
			err: &url.Error{
				Op: "Post", URL: "https://destination",
				Err: errors.New("the proxy refused the CONNECT with status 407"),
			},
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
