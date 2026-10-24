// Package gateway carries the HTTP transport for the builder's chat completions.
//
// @file      internal/gateway/gateway_test.go
// @for       The request shape and the error path of the chat-completions client.
// @uses      encoding/json, net/http, net/http/httptest, testing, time.
// @reason    The bearer key and the endpoint are security sensitive, so a local server pins both instead of a live gateway.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-23
package gateway_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/gateway"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/masterprompt"
)

func TestComplete_GivenVariations_ThenItSendsAndParses(t *testing.T) {
	var (
		seenPath  string
		seenAuth  string
		seenModel string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		seenModel, _ = body["model"].(string)
		if r.URL.Path != "/api/v1/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"the-model","choices":[{"message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`))
	}))
	defer server.Close()

	client := gateway.New(server.URL, "secret-key", 5*time.Second)
	result, err := client.Complete(context.Background(), masterprompt.ChatRequest{Model: "the-model", System: "s", User: "u", MaxTokens: 700, Temperature: 0.2})
	if err != nil {
		t.Fatalf("Complete() error = %v, want nil", err)
	}
	if seenPath != "/api/v1/chat/completions" {
		t.Fatalf("path = %q, want the completions endpoint", seenPath)
	}
	if seenAuth != "Bearer secret-key" {
		t.Fatalf("authorization = %q, want the bearer key", seenAuth)
	}
	if seenModel != "the-model" {
		t.Fatalf("model = %q, want the requested model", seenModel)
	}
	if result.ModelID != "the-model" || result.Usage.TotalTokens != 30 {
		t.Fatalf("result = %+v, want the model id and usage to come back", result)
	}
	if !strings.Contains(result.Content, `"ok"`) {
		t.Fatalf("content = %q, want the raw document text", result.Content)
	}
}

func TestComplete_GivenANonSuccessStatus_ThenItErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream is down", http.StatusBadGateway)
	}))
	defer server.Close()

	client := gateway.New(server.URL, "secret-key", 5*time.Second)
	_, err := client.Complete(context.Background(), masterprompt.ChatRequest{Model: "the-model", System: "s", User: "u"})
	if err == nil {
		t.Fatal("Complete() error = nil, want one for a 5xx response")
	}
}
