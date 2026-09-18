// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/endpoint_test.go
// @for       Table-driven tests for the upstream URL a connector builds.
// @uses      testing, internal/registry.
// @reason    The registry stores a complete chat URL rather than a base, because
//
//	providers disagree about where the path ends and several add a
//	suffix. The join rule is therefore "use the entry's URL and append
//	what it declares", and every shape it has to survive is pinned
//	here so the core never has to guess a path. A declared chat_path
//	is the one entry that says its base_url is a base, so the join
//	has to tolerate both readings.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package provider

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

func TestDefault_EndpointBuildsTheURL(t *testing.T) {
	cases := []struct {
		name      string
		transport registry.Transport
		wantURL   string
		wantErr   bool
	}{
		{
			name:      "a full chat url is used as written",
			transport: registry.Transport{BaseURL: "https://api.deepseek.com/chat/completions"},
			wantURL:   "https://api.deepseek.com/chat/completions",
		},
		{
			name:      "a query suffix is appended",
			transport: registry.Transport{BaseURL: "https://api.anthropic.com/v1/messages", URLSuffix: "?beta=true"},
			wantURL:   "https://api.anthropic.com/v1/messages?beta=true",
		},
		{
			name: "a responses url wins for the responses format",
			transport: registry.Transport{
				Format: registry.FormatOpenAIResponses, BaseURL: "https://chat.example.test/chat",
				ResponsesURL: "https://responses.example.test/v1/responses",
			},
			wantURL: "https://responses.example.test/v1/responses",
		},
		{
			name:      "a declared chat path completes a base",
			transport: registry.Transport{BaseURL: "https://api2.cursor.sh", ChatPath: "/aiserver.v1.ChatService/StreamUnifiedChatWithTools"},
			wantURL:   "https://api2.cursor.sh/aiserver.v1.ChatService/StreamUnifiedChatWithTools",
		},
		{
			name:      "a trailing slash on the base is not doubled",
			transport: registry.Transport{BaseURL: "https://upstream.test/v1/", ChatPath: "/chat/completions"},
			wantURL:   "https://upstream.test/v1/chat/completions",
		},
		{
			name:      "a path without a leading slash is joined the same way",
			transport: registry.Transport{BaseURL: "https://upstream.test/v1", ChatPath: "chat/completions"},
			wantURL:   "https://upstream.test/v1/chat/completions",
		},
		{
			name:      "a suffix lands after the declared path, not before it",
			transport: registry.Transport{BaseURL: "https://upstream.test/v1", ChatPath: "/chat/completions", URLSuffix: "?beta=true"},
			wantURL:   "https://upstream.test/v1/chat/completions?beta=true",
		},
		{
			name:      "an entry with no chat path keeps its full url as written",
			transport: registry.Transport{BaseURL: "https://api.deepseek.com/v1/chat/completions", ChatPath: ""},
			wantURL:   "https://api.deepseek.com/v1/chat/completions",
		},
		{
			name:      "the first of several hosts is the primary",
			transport: registry.Transport{BaseURLs: []string{"https://primary.test/v1", "https://secondary.test/v1"}},
			wantURL:   "https://primary.test/v1",
		},
		{
			name:      "base_url wins over base_urls when both are declared",
			transport: registry.Transport{BaseURL: "https://chosen.test/v1", BaseURLs: []string{"https://other.test/v1"}},
			wantURL:   "https://chosen.test/v1",
		},
		{
			name:      "a plain http url is accepted for a self-hosted upstream",
			transport: registry.Transport{BaseURL: "http://localhost:8081/v1/chat/completions"},
			wantURL:   "http://localhost:8081/v1/chat/completions",
		},
		{
			name:      "no url at all is an error",
			transport: registry.Transport{},
			wantErr:   true,
		},
		{
			name:      "a relative url is an error",
			transport: registry.Transport{BaseURL: "/v1/chat/completions"},
			wantErr:   true,
		},
		{
			name:      "a scheme-less host is an error",
			transport: registry.Transport{BaseURL: "api.example.test/v1"},
			wantErr:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := registry.Provider{ID: "p", Transport: tc.transport}
			connector := NewDefault(entry)
			got, err := connector.Endpoint(Request{Provider: entry}, Credential{APIKey: "sk-x"})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Endpoint() = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Endpoint() error = %v", err)
			}
			if got != tc.wantURL {
				t.Fatalf("Endpoint() = %q, want %q", got, tc.wantURL)
			}
		})
	}
}
