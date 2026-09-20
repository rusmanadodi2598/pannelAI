// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_auth_test.go
// @for       The §4 data-plane credential extraction rule.
// @uses      net/http, net/http/httptest, testing.
// @reason    The data plane accepts the gateway key from either credential
//
//	header, because OpenAI-shaped clients send Authorization: Bearer
//	while Anthropic-shaped clients send X-Api-Key. The extraction is
//	the one place that rule lives, so a table pins it: a request built
//	from each row must yield exactly the token that row carries, and a
//	request with neither header yields nothing, which is what makes the
//	later refusal a 401 rather than a silent skip.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerTokenExtraction(t *testing.T) {
	cases := []struct {
		name          string
		authorization string
		apiKey        string
		want          string
	}{
		{name: "the OpenAI header", authorization: "Bearer sk-live1", want: "sk-live1"},
		{name: "a lowercase scheme", authorization: "bearer sk-live1", want: "sk-live1"},
		{name: "padding inside the header", authorization: "Bearer  sk-live1 ", want: "sk-live1"},
		{name: "the Anthropic header", apiKey: "sk-live2", want: "sk-live2"},
		{name: "the OpenAI header wins when both are sent", authorization: "Bearer sk-live1", apiKey: "sk-live2", want: "sk-live1"},
		{name: "a foreign scheme falls through to the Anthropic header", authorization: "Basic dXNlcjpwYXNz", apiKey: "sk-live2", want: "sk-live2"},
		{name: "an empty Authorization header falls through", authorization: "", apiKey: "sk-live2", want: "sk-live2"},
		{name: "no credential anywhere", want: ""},
		{name: "empty header values", authorization: " ", apiKey: " ", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", nil)
			if tc.authorization != "" {
				request.Header.Set("Authorization", tc.authorization)
			}
			if tc.apiKey != "" {
				request.Header.Set("X-Api-Key", tc.apiKey)
			}
			if got := bearerToken(request); got != tc.want {
				t.Fatalf("bearerToken() = %q, want %q", got, tc.want)
			}
		})
	}
}
