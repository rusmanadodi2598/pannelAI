// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_call_test.go
// @for       The model-string and base-URL resolution every §7.10 media route
//
//	shares.
//
// @uses      internal/domain, context, strings, testing.
// @reason    What a model string addresses and which base URL is in force are
//
//	the rules a route must not be able to drift on, and each is a silent
//	failure if wrong. The perform-side rules live in media_perform_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestMediaCallService_Prepare pins every refusal and every resolution the
// pipeline performs before a request is built.
func TestMediaCallService_Prepare(t *testing.T) {
	cases := []struct {
		name          string
		model         string
		kind          domain.MediaKind
		override      map[string]string
		wantProvider  string
		wantModel     string
		wantBaseURL   string
		wantErrorText string
	}{
		{
			name: "the registry's own base URL", model: "openai/gpt-4o-mini-tts", kind: domain.MediaKindTTS,
			wantProvider: "openai", wantModel: "gpt-4o-mini-tts", wantBaseURL: "https://api.openai.com/v1/audio/speech",
		},
		{
			name: "the default model when the client names none", model: "openai/", kind: domain.MediaKindTTS,
			wantProvider: "openai", wantModel: "gpt-4o-mini-tts", wantBaseURL: "https://api.openai.com/v1/audio/speech",
		},
		{
			name: "a stored override wins", model: "openai/gpt-4o-mini-tts", kind: domain.MediaKindTTS,
			override:     map[string]string{"openai\x00tts": "http://127.0.0.1:8000/v1"},
			wantProvider: "openai", wantModel: "gpt-4o-mini-tts", wantBaseURL: "http://127.0.0.1:8000/v1",
		},
		{
			name: "a model id that carries a slash", model: "openai/gpt-4o-mini-tts",
			kind: domain.MediaKindTTS, wantProvider: "openai", wantModel: "gpt-4o-mini-tts",
			wantBaseURL: "https://api.openai.com/v1/audio/speech",
		},
		{
			name: "an unknown provider", model: "ghost/tts-1", kind: domain.MediaKindTTS,
			wantErrorText: "is not in the registry",
		},
		{
			name: "a bare model with no provider", model: "gpt-4o-mini-tts", kind: domain.MediaKindTTS,
			wantErrorText: "expect provider/model",
		},
		{
			name: "a kind the provider does not offer", model: "elevenlabs/eleven_multilingual_v2",
			kind: domain.MediaKindImage, wantErrorText: "does not offer image",
		},
		{
			name: "a media format the gateway does not translate", model: "elevenlabs/eleven_multilingual_v2",
			kind: domain.MediaKindTTS, wantErrorText: "elevenlabs tts format",
		},
		{
			name: "a provider with no base URL", model: "selfhosted/sd-xl", kind: domain.MediaKindImage,
			wantErrorText: "has no image base_url configured",
		},
		{
			name: "a search provider needs no model", model: "brave-search/", kind: domain.MediaKindSearch,
			wantProvider: "brave-search", wantBaseURL: "https://api.search.brave.com/res/v1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := mediaCallFixture(t)
			if tc.override != nil {
				svc.overrides = stubOverrideReader{values: tc.override}
			}
			call, err := svc.Prepare(context.Background(), tc.model, tc.kind, nil)
			if tc.wantErrorText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrorText) {
					t.Fatalf("error = %v, want it to mention %q", err, tc.wantErrorText)
				}
				return
			}
			if err != nil {
				t.Fatalf("Prepare() error = %v", err)
			}
			if call.ProviderID != tc.wantProvider || call.UpstreamModel != tc.wantModel {
				t.Fatalf("call = %s/%s, want %s/%s", call.ProviderID, call.UpstreamModel, tc.wantProvider, tc.wantModel)
			}
			if call.BaseURL != tc.wantBaseURL {
				t.Fatalf("base URL = %q, want %q", call.BaseURL, tc.wantBaseURL)
			}
		})
	}
}

// TestMediaCallService_PrepareTargets pins the credential placement the kind
// declares, including the query-parameter form (§8.1).
func TestMediaCallService_PrepareTargets(t *testing.T) {
	svc, _, _ := mediaCallFixture(t)
	call, err := svc.Prepare(context.Background(), "openai/gpt-4o-mini-tts", domain.MediaKindTTS, nil)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if call.Target != "https://api.openai.com/v1/audio/speech" {
		t.Fatalf("target = %q, want the declared base URL", call.Target)
	}
	if call.Headers["Content-Type"] != "application/json" {
		t.Fatalf("headers = %v, want a JSON content type", call.Headers)
	}
	if call.Selection.Endpoint.ID() != "ep-openai" {
		t.Fatalf("endpoint = %q, want the selected one", call.Selection.Endpoint.ID())
	}
}
