// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_filter_test.go
// @for       That GET /api/v1/endpoints refuses an unknown status and an over-long provider id before calling the service.
// @uses      net/http, strings, testing, internal/schema.
// @reason    The status filter used to reach the repository as an arbitrary string, where a misspelling narrowed nothing and read as an empty page. The handler is built with no service because a refused filter must stop before the first call.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-22
package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestEndpointHandler_ListRejectsAStaleFilter pins the closed status set and the
// provider id ceiling.
func TestEndpointHandler_ListRejectsAStaleFilter(t *testing.T) {
	h := NewEndpointHandler(nil)

	cases := []struct {
		name   string
		target string
		field  string
	}{
		{name: "an unknown status", target: "/api/v1/endpoints?status=paused", field: "Status"},
		{name: "an over-long provider id", target: "/api/v1/endpoints?provider_id=" + strings.Repeat("p", 65), field: "ProviderID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, http.MethodGet, tc.target, "", h.List)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), `"VALIDATION_ERROR"`) ||
				!strings.Contains(rr.Body.String(), tc.field) {
				t.Fatalf("body = %s, want a VALIDATION_ERROR naming %s", rr.Body.String(), tc.field)
			}
		})
	}
}

// TestEndpointListQuery_AcceptsTheLifecycleSet guards the values the filter
// exists for, including the error state the health worker writes.
func TestEndpointListQuery_AcceptsTheLifecycleSet(t *testing.T) {
	for _, status := range []string{"", "active", "disabled", "error"} {
		query := schema.EndpointListQuery{Status: status}
		if err := schema.ValidateStruct(query); err != nil {
			t.Fatalf("status %q refused: %v", status, err)
		}
	}
}
