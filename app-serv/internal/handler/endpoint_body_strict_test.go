// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_body_strict_test.go
// @for       The strict decode shared by the routes whose body may be absent.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    The optional-body routes still accept a typed body, so an unknown member or a second JSON value must be refused before the service sees a partial request.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-22
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeIntoRefusesAnUnknownField(t *testing.T) {
	var body struct {
		Known string `json:"known"`
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"known":"a","extra":true}`))
	if err := decodeInto(request, &body); err == nil {
		t.Fatal("decodeInto() accepted an unknown field")
	}
}

func TestDecodeIntoRefusesTrailingData(t *testing.T) {
	var body struct {
		Known string `json:"known"`
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"known":"a"}{"known":"b"}`))
	if err := decodeInto(request, &body); err == nil {
		t.Fatal("decodeInto() accepted a second JSON value")
	}
}

func TestDecodeIntoAcceptsTheDeclaredShape(t *testing.T) {
	var body struct {
		Known string `json:"known"`
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"known":"a"}`))
	if err := decodeInto(request, &body); err != nil {
		t.Fatalf("decodeInto() error = %v", err)
	}
	if body.Known != "a" {
		t.Fatalf("known = %q, want a", body.Known)
	}
}
