// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/proxy_test.go
// @for       The Proxy aggregate's shape rules and transitions (SPEC-API-001 §7.11).
// @uses      testing, strings, time, net/netip.
// @reason    The host rule is a security boundary — it is what stops a URL, a
//
//	userinfo trick, or a decimal IP from reaching the resolver — so it is
//	pinned with a table that varies the obfuscation rather than one
//	example (OWASP A01 §2.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-19
package domain

import (
	"strings"
	"testing"
	"time"
)

func proxyNow() time.Time { return time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC) }

// TestNewProxy_HostShape pins the host rule against the shapes an SSRF attempt
// uses to move the resolver's target: a scheme, userinfo, a path, a port, and
// the numeric spellings of an IPv4 address. The benign controls are the shapes
// an operator really writes.
func TestNewProxy_HostShape(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		wantErr string
	}{
		{name: "a public hostname", host: "proxy.example.com"},
		{name: "a private address literal", host: "192.168.1.10"},
		{name: "a public address literal", host: "203.0.113.7"},
		{name: "an IPv6 literal", host: "2606:4700:4700::1111"},
		{name: "a URL", host: "http://proxy.example.com", wantErr: "bare hostname"},
		{name: "userinfo", host: "proxy.example.com@evil.test", wantErr: "bare hostname"},
		{name: "a path", host: "proxy.example.com/pool", wantErr: "bare hostname"},
		{name: "an embedded port", host: "proxy.example.com:8080", wantErr: "bare hostname"},
		{name: "a decimal IPv4 address", host: "2130706433", wantErr: "dotted form"},
		{name: "an octal IPv4 address", host: "0177.0.0.1", wantErr: "dotted form"},
		{name: "a shortened IPv4 address", host: "127.1", wantErr: "dotted form"},
		{name: "a scheme-only value", host: "socks5://", wantErr: "bare hostname"},
		{name: "an empty host", host: "   ", wantErr: "host is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewProxy("", "pool", ProxyProtocolHTTP, tc.host, 8080, "", "", proxyNow())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("NewProxy(host=%q) error = %v, want accepted", tc.host, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("NewProxy(host=%q) accepted a shape that must be refused", tc.host)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewProxy(host=%q) error = %v, want it to mention %q", tc.host, err, tc.wantErr)
			}
		})
	}
}

// TestParseProxyProtocol pins the closed protocol set.
func TestParseProxyProtocol(t *testing.T) {
	cases := []struct {
		raw     string
		want    ProxyProtocol
		wantErr bool
	}{
		{raw: "http", want: ProxyProtocolHTTP},
		{raw: "https", want: ProxyProtocolHTTPS},
		{raw: "socks5", want: ProxyProtocolSOCKS5},
		{raw: " SOCKS5 ", want: ProxyProtocolSOCKS5},
		{raw: "socks4", wantErr: true},
		{raw: "ftp", wantErr: true},
		{raw: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseProxyProtocol(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseProxyProtocol(%q) accepted an unknown protocol", tc.raw)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("ParseProxyProtocol(%q) = %q, %v; want %q", tc.raw, got, err, tc.want)
			}
		})
	}
}

// TestNewProxy_RejectsAnUnknownProtocol pins that the constructor defends its
// own invariant: a value cast past the parser is still refused.
func TestNewProxy_RejectsAnUnknownProtocol(t *testing.T) {
	if _, err := NewProxy("", "pool", ProxyProtocol("ftp"), "proxy.example.com", 3128, "", "", proxyNow()); err == nil {
		t.Fatal("NewProxy() accepted a protocol outside the closed set")
	}
}

// TestNewProxy_Fields pins the remaining constructor rules and the id prefix.
func TestNewProxy_Fields(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		label   string
		port    int
		user    string
		wantErr string
	}{
		{name: "a bare id gains the proxy prefix", id: "candidate-1", label: "pool", port: 3128, wantErr: ""},
		{name: "a prefixed id is kept", id: IDPrefixProxy + "candidate-2", label: "pool", port: 3128},
		{name: "a missing label", label: " ", port: 3128, wantErr: "label is required"},
		{name: "an over-long label", label: strings.Repeat("x", 121), port: 3128, wantErr: "at most 120"},
		{name: "port zero", label: "pool", port: 0, wantErr: "between 1 and 65535"},
		{name: "port above the TCP range", label: "pool", port: 65536, wantErr: "between 1 and 65535"},
		{name: "an over-long username", label: "pool", port: 3128, user: strings.Repeat("u", 201), wantErr: "at most 200"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proxy, err := NewProxy(tc.id, tc.label, ProxyProtocolHTTP, "proxy.example.com", tc.port, tc.user, "sealed", proxyNow())
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("NewProxy() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("NewProxy() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProxy() error = %v", err)
			}
			if !strings.HasPrefix(proxy.ID(), IDPrefixProxy) {
				t.Fatalf("id = %q, want the %q prefix", proxy.ID(), IDPrefixProxy)
			}
			if !proxy.Enabled() || !proxy.HasPassword() {
				t.Fatalf("a new candidate must be enabled and carry its sealed password: %+v", proxy)
			}
		})
	}
}

// TestProxy_RepointAndCredentialsClearTheStatus pins that a status measured
// against an address or a credential that has since changed is not reported as
// if it still held.
func TestProxy_RepointAndCredentialsClearTheStatus(t *testing.T) {
	proxy, err := NewProxy("", "pool", ProxyProtocolHTTP, "proxy.example.com", 3128, "", "", proxyNow())
	if err != nil {
		t.Fatalf("NewProxy() error = %v", err)
	}
	proxy.RecordTest(EndpointTestOK, 12, "", proxyNow())

	if err := proxy.Repoint(ProxyProtocolSOCKS5, "other.example.com", 1080, proxyNow()); err != nil {
		t.Fatalf("Repoint() error = %v", err)
	}
	if proxy.Status().State != "" || proxy.Status().CheckedAt != nil {
		t.Fatalf("Repoint() kept the old status: %+v", proxy.Status())
	}
	if proxy.Protocol() != ProxyProtocolSOCKS5 || proxy.Host() != "other.example.com" || proxy.Port() != 1080 {
		t.Fatalf("Repoint() = %s/%s/%d, want socks5/other.example.com/1080", proxy.Protocol(), proxy.Host(), proxy.Port())
	}

	proxy.RecordTest(EndpointTestFail, 30, "the proxy refused the connection", proxyNow())
	if err := proxy.SetCredentials("operator", "sealed-2", proxyNow()); err != nil {
		t.Fatalf("SetCredentials() error = %v", err)
	}
	if proxy.Status().State != "" {
		t.Fatalf("SetCredentials() kept the old status: %+v", proxy.Status())
	}
	if proxy.Username() != "operator" || proxy.PasswordEncrypted() != "sealed-2" {
		t.Fatalf("SetCredentials() = %q/%q", proxy.Username(), proxy.PasswordEncrypted())
	}
}

// TestProxy_RecordTest pins the stored test result, including the timestamp the
// panel reads back.
func TestProxy_RecordTest(t *testing.T) {
	proxy, err := NewProxy("", "pool", ProxyProtocolHTTPS, "proxy.example.com", 8443, "", "", proxyNow())
	if err != nil {
		t.Fatalf("NewProxy() error = %v", err)
	}
	checked := proxyNow().Add(time.Minute)
	proxy.RecordTest(EndpointTestFail, 45, "the proxy answered 502", checked)

	status := proxy.Status()
	if status.State != EndpointTestFail || status.LatencyMS != 45 || status.Message != "the proxy answered 502" {
		t.Fatalf("RecordTest() = %+v", status)
	}
	if status.CheckedAt == nil || !status.CheckedAt.Equal(checked) {
		t.Fatalf("RecordTest() checked_at = %v, want %v", status.CheckedAt, checked)
	}
	if !proxy.UpdatedAt().Equal(checked) {
		t.Fatalf("RecordTest() updated_at = %v, want %v", proxy.UpdatedAt(), checked)
	}
}

// TestProxy_SetEnabledKeepsTheAddress pins that disabling is not a repoint.
func TestProxy_SetEnabledKeepsTheAddress(t *testing.T) {
	proxy, err := NewProxy("", "pool", ProxyProtocolHTTP, "proxy.example.com", 3128, "user", "sealed", proxyNow())
	if err != nil {
		t.Fatalf("NewProxy() error = %v", err)
	}
	proxy.SetEnabled(false, proxyNow())
	if proxy.Enabled() {
		t.Fatal("SetEnabled(false) left the candidate enabled")
	}
	if proxy.Host() != "proxy.example.com" || proxy.Port() != 3128 || proxy.Username() != "user" || !proxy.HasPassword() {
		t.Fatalf("SetEnabled() changed the address or credentials: %+v", proxy)
	}
}
