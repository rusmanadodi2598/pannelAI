// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/dto_strict_test.go
// @for       The strict half of DecodeJSON: one JSON value, no trailing data, and a JSON content type.
// @uses      net/http, strings, testing.
// @reason    A body whose first value parses can otherwise smuggle a second value or a non-JSON payload past the management routes, so the boundary is pinned rather than left to the decoder's first-read behavior.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-22
package schema

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRefusesTrailingData(t *testing.T) {
	request := httptestRequest(http.MethodPost, `{"name":"a"}{"name":"b"}`, "")
	var body struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(request, &body); err == nil {
		t.Fatal("DecodeJSON() accepted a second JSON value")
	}
}

func TestDecodeJSONRefusesANonJSONContentType(t *testing.T) {
	request := httptestRequest(http.MethodPost, `{"name":"a"}`, "text/plain")
	var body struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(request, &body); err == nil {
		t.Fatal("DecodeJSON() accepted text/plain")
	}
}

func TestDecodeJSONAcceptsOneJSONValue(t *testing.T) {
	request := httptestRequest(http.MethodPost, `{"name":"a"} `, "application/json; charset=utf-8")
	var body struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(request, &body); err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if body.Name != "a" {
		t.Fatalf("name = %q, want a", body.Name)
	}
}

func httptestRequest(method, body, contentType string) *http.Request {
	request := httptest.NewRequest(method, "/", strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request
}
