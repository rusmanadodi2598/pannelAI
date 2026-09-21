// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_models_test.go
// @for       The §7.15 models list Playground reads to fill its model selector.
// @uses      internal/dataplane, net/http, net/http/httptest, strings, testing.
// @reason    F4 and F6 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md
// require the route the panel depends on to prove its own contract: the OpenAI
// list shape, the auth gate, and an empty catalog that answers an empty list
// rather than a panic or a fabricated model.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// TestChatModelsHTTP_Table pins the models route as the panel consumes it: a
// valid gateway key receives the OpenAI list shape, and an invalid or absent one
// is refused with the data-plane envelope.
func TestChatModelsHTTP_Table(t *testing.T) {
	fixture := newChatHTTPFixture(t)
	handler := NewChatHandler(fixture.chat)
	cases := []struct {
		name       string
		key        string
		wantStatus int
		wantCode   string
	}{
		{name: "a valid key receives the list", key: fixture.key, wantStatus: http.StatusOK},
		{name: "an unknown key is refused", key: "sk-wrong", wantStatus: http.StatusUnauthorized, wantCode: dataplane.CodeUnauthorized},
		{name: "no key is refused", wantStatus: http.StatusUnauthorized, wantCode: dataplane.CodeUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
			if tc.key != "" {
				request.Header.Set("Authorization", "Bearer "+tc.key)
			}
			recorder := httptest.NewRecorder()
			handler.Models(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if tc.wantCode != "" && !strings.Contains(recorder.Body.String(), tc.wantCode) {
				t.Fatalf("body = %s, want code %s", recorder.Body.String(), tc.wantCode)
			}
			if tc.wantStatus == http.StatusOK {
				assertOpenAIListShape(t, recorder.Body.String())
			}
		})
	}
}

// TestChatModelsHTTP_EmptyCatalogIsAnEmptyList pins the state the panel renders
// before any provider is configured: the answer is a valid list with no entries,
// which is a state the panel can show, rather than a panic or an invented model.
func TestChatModelsHTTP_EmptyCatalogIsAnEmptyList(t *testing.T) {
	fixture := newChatHTTPFixture(t)
	handler := NewChatHandler(fixture.chat)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	recorder := httptest.NewRecorder()
	handler.Models(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	assertOpenAIListShape(t, recorder.Body.String())
	if !strings.Contains(recorder.Body.String(), `"data":[]`) {
		t.Fatalf("body = %s, want an empty data array rather than a fabricated model", recorder.Body.String())
	}
}

// assertOpenAIListShape pins the two members the panel reads: the list object
// marker and a data array, which is what an OpenAI-shaped client expects.
func assertOpenAIListShape(t *testing.T, body string) {
	t.Helper()
	for _, member := range []string{`"object":"list"`, `"data":`} {
		if !strings.Contains(body, member) {
			t.Errorf("body = %s, want the OpenAI list shape member %s", body, member)
		}
	}
}
