// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/proxy_probe_test.go
// @for       HTTP tests for the two §7.11 connectivity-test routes.
// @uses      net/http, testing, internal/domain, internal/service.
// @reason    The two routes answer the same status but differ on one rule —
//
//	the stored route persists its finding and the candidate route must
//	not — so the tests pin the shared answer shape and the difference.
//	A failed probe is a 200 carrying a fail state: that is the answer
//	the operator asked for, not an error.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TestProxyHandler_Test_StoredCandidate pins that the route answers with the
// probe's finding and stores it, so the list reports the same result.
func TestProxyHandler_Test_StoredCandidate(t *testing.T) {
	f := newManagementFixture(t)
	id := createProxy(t, f, "pool-a")

	rr := do(t, http.MethodPost, "/api/v1/proxies/"+id+"/test", "", withPathID(f.proxy.Test, id))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["state"] != string(domain.EndpointTestOK) || body["latency_ms"] != float64(7) {
		t.Fatalf("state/latency = %v/%v, want ok/7", body["state"], body["latency_ms"])
	}
	if checkedAt, _ := body["checked_at"].(string); checkedAt == "" {
		t.Fatalf("checked_at = %q, want a timestamp", checkedAt)
	}

	listed, _ := decodeBody(t, do(t, http.MethodGet, "/api/v1/proxies", "", f.proxy.List))["data"].([]any)
	entry, _ := listed[0].(map[string]any)
	status, _ := entry["status"].(map[string]any)
	if status["state"] != string(domain.EndpointTestOK) {
		t.Fatalf("stored status = %v, want the probe's finding persisted", entry["status"])
	}
}

// TestProxyHandler_Test_AFailureIsAResult pins that a failed probe answers 200
// with the failure in the body, and that the failure carries its reason.
func TestProxyHandler_Test_AFailureIsAResult(t *testing.T) {
	f := newManagementFixture(t)
	f.proxyProber.result = service.ProxyProbeResult{
		State: domain.EndpointTestFail, Message: "the proxy refused the connection",
	}
	id := createProxy(t, f, "pool-a")

	rr := do(t, http.MethodPost, "/api/v1/proxies/"+id+"/test", "", withPathID(f.proxy.Test, id))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["state"] != string(domain.EndpointTestFail) || body["message"] != "the proxy refused the connection" {
		t.Fatalf("state/message = %v/%v, want the failure reported", body["state"], body["message"])
	}
}

// TestProxyHandler_Test_UnknownID pins the route's only error: an unreadable
// candidate has nothing to probe.
func TestProxyHandler_Test_UnknownID(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodPost, "/api/v1/proxies/prx_missing/test", "", withPathID(f.proxy.Test, "prx_missing"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestProxyHandler_TestCandidate pins the unsaved-candidate route: the answer
// carries the finding and nothing is stored.
func TestProxyHandler_TestCandidate(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodPost, "/api/v1/proxies/test",
		`{"protocol":"socks5","host":"proxy.example.com","port":1080,`+proxyCredentials+`}`, f.proxy.TestCandidate)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if state, _ := decodeBody(t, rr)["state"].(string); state != string(domain.EndpointTestOK) {
		t.Fatalf("state = %q, want ok", state)
	}
	if len(f.proxyRepo.byID) != 0 {
		t.Fatalf("the candidate route stored %d rows, want none", len(f.proxyRepo.byID))
	}
	if len(f.proxyProber.targets) != 1 {
		t.Fatalf("prober calls = %d, want one", len(f.proxyProber.targets))
	}
	target := f.proxyProber.targets[0]
	if target.Protocol != domain.ProxyProtocolSOCKS5 || target.Host != "proxy.example.com" || target.Port != 1080 {
		t.Fatalf("target = %+v, want the submitted candidate", target)
	}
	if target.Password != "s3cret-value" {
		t.Fatalf("target password = %q, want the submitted plaintext", target.Password)
	}
}

// TestProxyHandler_TestCandidate_Validation pins the candidate route's
// rejections.
func TestProxyHandler_TestCandidate_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "an unknown protocol", body: `{"protocol":"ftp","host":"proxy.example.com","port":3128}`},
		{name: "a missing host", body: `{"protocol":"http","port":3128}`},
		{name: "a port outside the range", body: `{"protocol":"http","host":"proxy.example.com","port":70000}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPost, "/api/v1/proxies/test", tc.body, f.proxy.TestCandidate)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if len(f.proxyProber.targets) != 0 {
				t.Fatalf("a rejected candidate reached the prober: %+v", f.proxyProber.targets)
			}
		})
	}
}
