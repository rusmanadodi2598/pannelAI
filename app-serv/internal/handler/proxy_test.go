// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/proxy_test.go
// @for       HTTP tests for the §7.11 proxy pool CRUD routes.
// @uses      net/http, strconv, strings, testing.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path
//
//	per route, and §7.11 makes the password write-only: the happy path
//	asserts the plaintext never comes back, which is the one property a
//	response-shape change would silently break.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// proxyBody builds a create or patch body for the pool routes.
func proxyBody(label, protocol, host string, port int, extra string) string {
	body := `{"label":"` + label + `","protocol":"` + protocol + `","host":"` + host + `","port":` + strconv.Itoa(port)
	if extra != "" {
		body += "," + extra
	}
	return body + "}"
}

// proxyCredentials is the valid extra pair most cases send.
const proxyCredentials = `"username":"operator","password":"s3cret-value"`

// createProxy stores a candidate through the route and returns its id.
func createProxy(t *testing.T, f managementFixture, label string) string {
	t.Helper()
	rr := do(t, http.MethodPost, "/api/v1/proxies",
		proxyBody(label, "http", "proxy.example.com", 3128, proxyCredentials), f.proxy.Create)
	if rr.Code != http.StatusCreated {
		t.Fatalf("seeding %s: status = %d (body: %s)", label, rr.Code, rr.Body.String())
	}
	id, _ := decodeBody(t, rr)["id"].(string)
	return id
}

// TestProxyHandler_Create_HappyPath pins the accepted shape, the prx_ id, and
// that the response never echoes the password.
func TestProxyHandler_Create_HappyPath(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodPost, "/api/v1/proxies",
		proxyBody("pool-a", "http", "proxy.example.com", 3128, proxyCredentials), f.proxy.Create)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "s3cret-value") {
		t.Fatalf("the response echoed the password: %s", rr.Body.String())
	}

	body := decodeBody(t, rr)
	if id, _ := body["id"].(string); !strings.HasPrefix(id, "prx_") {
		t.Fatalf("id = %q, want the prx_ prefix", id)
	}
	if body["has_password"] != true || body["enabled"] != true {
		t.Fatalf("has_password/enabled = %v/%v, want true/true", body["has_password"], body["enabled"])
	}
	if _, ok := body["status"]; ok {
		t.Fatalf("an untested candidate must carry no status: %v", body)
	}
}

// TestProxyHandler_Create_Validation pins every rejection the route owes its
// client: the schema failures and the host-shape rule.
func TestProxyHandler_Create_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "a missing label", body: `{"protocol":"http","host":"proxy.example.com","port":3128}`},
		{name: "an unknown protocol", body: proxyBody("pool", "ftp", "proxy.example.com", 3128, "")},
		{name: "a port below the range", body: proxyBody("pool", "http", "proxy.example.com", 0, "")},
		{name: "a port above the range", body: proxyBody("pool", "http", "proxy.example.com", 70000, "")},
		{name: "a URL in the host field", body: proxyBody("pool", "http", "http://proxy.example.com", 3128, "")},
		{name: "a missing host", body: `{"label":"pool","protocol":"http","port":3128}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPost, "/api/v1/proxies", tc.body, f.proxy.Create)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			errorBody, _ := decodeBody(t, rr)["error"].(map[string]any)
			if errorBody["code"] != "VALIDATION_ERROR" {
				t.Fatalf("error = %v, want VALIDATION_ERROR", errorBody)
			}
		})
	}
}

// TestProxyHandler_List pins the whole-pool answer and its label order.
func TestProxyHandler_List(t *testing.T) {
	f := newManagementFixture(t)
	createProxy(t, f, "beta")
	createProxy(t, f, "alpha")

	rr := do(t, http.MethodGet, "/api/v1/proxies", "", f.proxy.List)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	data, ok := decodeBody(t, rr)["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data = %v, want two candidates", data)
	}
	first, _ := data[0].(map[string]any)
	if first["label"] != "alpha" {
		t.Fatalf("first label = %v, want alpha (ordered)", first["label"])
	}
}

// TestProxyHandler_Update pins the partial patch: an omitted field keeps its
// value, a full body still applies, and an omitted password keeps the secret.
func TestProxyHandler_Update(t *testing.T) {
	f := newManagementFixture(t)
	id := createProxy(t, f, "pool-a")

	rr := do(t, http.MethodPatch, "/api/v1/proxies/"+id,
		proxyBody("pool-b", "http", "proxy.example.com", 3128, ""), withPathID(f.proxy.Update, id))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["label"] != "pool-b" || body["has_password"] != true {
		t.Fatalf("label/has_password = %v/%v, want pool-b/true", body["label"], body["has_password"])
	}
}

// TestProxyHandler_Update_Partial pins the label-only patch: omitted fields
// keep their stored values instead of failing validation.
func TestProxyHandler_Update_Partial(t *testing.T) {
	f := newManagementFixture(t)
	id := createProxy(t, f, "pool-a")

	rr := do(t, http.MethodPatch, "/api/v1/proxies/"+id,
		`{"label":"pool-b"}`, withPathID(f.proxy.Update, id))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["label"] != "pool-b" {
		t.Fatalf("label = %v, want pool-b", body["label"])
	}
	if body["host"] != "proxy.example.com" || body["has_password"] != true {
		t.Fatalf("host/has_password = %v/%v, want the omitted fields kept", body["host"], body["has_password"])
	}
}

// TestProxyHandler_Update_Validation pins the patch refusals: an empty patch
// names nothing to change, and an unknown protocol is rejected.
func TestProxyHandler_Update_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "an empty patch", body: `{}`},
		{name: "an unknown protocol", body: `{"label":"pool","protocol":"ftp"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			id := createProxy(t, f, "pool-a")
			rr := do(t, http.MethodPatch, "/api/v1/proxies/"+id, tc.body, withPathID(f.proxy.Update, id))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			errorBody, _ := decodeBody(t, rr)["error"].(map[string]any)
			if errorBody["code"] != "VALIDATION_ERROR" {
				t.Fatalf("error = %v, want VALIDATION_ERROR", errorBody)
			}
		})
	}
}

// TestProxyHandler_Update_UnknownID pins the not-found mapping.
func TestProxyHandler_Update_UnknownID(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodPatch, "/api/v1/proxies/prx_missing",
		proxyBody("pool", "http", "proxy.example.com", 3128, ""), withPathID(f.proxy.Update, "prx_missing"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestProxyHandler_Delete pins removal and the second delete's not-found.
func TestProxyHandler_Delete(t *testing.T) {
	f := newManagementFixture(t)
	id := createProxy(t, f, "pool-a")

	rr := do(t, http.MethodDelete, "/api/v1/proxies/"+id, "", withPathID(f.proxy.Delete, id))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("a 204 must carry no body: %s", rr.Body.String())
	}
	rr = do(t, http.MethodDelete, "/api/v1/proxies/"+id, "", withPathID(f.proxy.Delete, id))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestProxyHandler_MissingID covers the path-value guard on the routes that
// address one candidate.
func TestProxyHandler_MissingID(t *testing.T) {
	f := newManagementFixture(t)
	cases := []struct {
		name   string
		method string
		fn     http.HandlerFunc
	}{
		{name: "patch", method: http.MethodPatch, fn: f.proxy.Update},
		{name: "delete", method: http.MethodDelete, fn: f.proxy.Delete},
		{name: "test", method: http.MethodPost, fn: f.proxy.Test},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, tc.method, "/api/v1/proxies/", "", tc.fn)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
		})
	}
}
