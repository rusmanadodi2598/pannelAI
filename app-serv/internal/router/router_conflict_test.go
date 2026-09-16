// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_conflict_test.go
// @for       The duplicate-name contract: two POSTs with the same name must conflict.
// @uses      internal/router, internal/schema, net/http, net/http/httptest,
//
//	strings, encoding/json, testing.
//
// @reason    This proves the handler and service map ErrGatewayKeyExists to a
//
//	409 with the §8 envelope. It cannot prove the UNIQUE index exists,
//	because it runs against a stub; that is
//	TestIntegration_Create_RejectsDuplicateName's job, against real
//	PostgreSQL. The stubs here mirror the index's rule so they do not
//	accept what the database would reject.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestRoutes_DuplicateNameIsConflict posts the same name twice through the real
// mux and asserts the second attempt is a 409 CONFLICT, which is the mapping
// from domain.ErrGatewayKeyExists to the wire. The stub stands in for the
// database here, so the existence of the UNIQUE index itself is proven by the
// integration test instead.
func TestRoutes_DuplicateNameIsConflict(t *testing.T) {
	mux := newTestRouter(t)

	const body = `{"name":"duplicate"}`
	first := httptest.NewRecorder()
	mux.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(body)))
	if first.Code != http.StatusCreated {
		t.Fatalf("first create = %d, want 201 (body: %s)", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	mux.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(body)))
	if second.Code != http.StatusConflict {
		t.Fatalf("second create = %d, want 409 (body: %s)", second.Code, second.Body.String())
	}

	var envelope schema.ErrorBody
	if err := json.NewDecoder(second.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Error.Code != "CONFLICT" {
		t.Fatalf("code = %q, want CONFLICT", envelope.Error.Code)
	}
}
