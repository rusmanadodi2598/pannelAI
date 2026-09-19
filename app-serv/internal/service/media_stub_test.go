// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_stub_test.go
// @for       The caller, router, and registry the §7.10 media call tests drive.
// @uses      internal/dataplane, internal/domain, internal/registry, context,
//
//	testing, time.
//
// @reason    The media call service talks to the outside world through three
//
//	seams — the media caller, the router port, and the registry index — so
//	all three are doubles here and each test is about one rule.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stubMediaCaller records every outbound request and answers with one canned
// response, so a test can assert both what was sent and what came back.
type stubMediaCaller struct {
	requests []dataplane.MediaRequest
	answer   dataplane.MediaResponse
	failure  error
}

func (c *stubMediaCaller) Do(_ context.Context, request dataplane.MediaRequest) (dataplane.MediaResponse, error) {
	c.requests = append(c.requests, request)
	if c.failure != nil {
		return dataplane.MediaResponse{}, c.failure
	}
	return c.answer, nil
}

// stubMediaRouter answers selection with one fixed endpoint and records the
// health reports a call produces.
type stubMediaRouter struct {
	failure    error
	credential provider.Credential
	successes  int
	failures   []string
}

func (r *stubMediaRouter) Select(_ context.Context, providerID string) (dataplane.Selection, error) {
	if r.failure != nil {
		return dataplane.Selection{}, r.failure
	}
	endpoint, err := domain.NewUpstreamEndpoint("ep-"+providerID, providerID, "primary",
		domain.UpstreamAuthAPIKey, 1, time.Now().UTC())
	if err != nil {
		return dataplane.Selection{}, err
	}
	return dataplane.Selection{Endpoint: endpoint, Credential: r.credential}, nil
}

func (r *stubMediaRouter) RecordSuccess(context.Context, dataplane.Selection) error {
	r.successes++
	return nil
}

func (r *stubMediaRouter) RecordFailure(_ context.Context, _ dataplane.Selection, reason string) error {
	r.failures = append(r.failures, reason)
	return nil
}

// mediaCallFixture builds the service over the doubles. The index holds one
// OpenAI-shaped provider, one provider whose speech format is not translated,
// and one whose block declares no base URL.
func mediaCallFixture(t *testing.T) (*MediaCallService, *stubMediaCaller, *stubMediaRouter) {
	t.Helper()
	caller := &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}}
	router := &stubMediaRouter{}
	svc, err := NewMediaCallService(MediaCallServiceDeps{
		Index:  &mediaIndex{entries: mediaCallEntries()},
		Router: router,
		Caller: caller,
	})
	if err != nil {
		t.Fatalf("NewMediaCallService() error = %v", err)
	}
	return svc, caller, router
}

// mediaCallEntries is the registry the media call tests read.
func mediaCallEntries() []registry.Provider {
	return []registry.Provider{
		{
			ID: "openai", Display: registry.Display{Name: "OpenAI"}, Priority: 1, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.openai.com/v1/audio/speech", AuthType: registry.AuthAPIKey,
					AuthHeader: "bearer", Format: "openai", DefaultModel: "gpt-4o-mini-tts",
					Voices: []registry.MediaVoice{{ID: "alloy", Name: "Alloy"}},
				},
				registry.MediaSTT: {
					BaseURL: "https://api.openai.com/v1/audio/transcriptions", Format: "openai",
				},
				registry.MediaImage: {
					BaseURL: "https://api.openai.com/v1/images/generations", Format: "openai",
					DefaultModel: "gpt-image-1",
				},
			},
		},
		{
			ID: "deepgram", Alias: "dg", Display: registry.Display{Name: "Deepgram"}, Priority: 2, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaSTT: {
					BaseURL: "https://api.deepgram.com/v1/listen", AuthType: registry.AuthAPIKey,
					AuthHeader: "token", Format: "deepgram",
				},
			},
		},
		{
			ID: "nvidia", Display: registry.Display{Name: "NVIDIA"}, Priority: 3, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL:  "https://integrate.api.nvidia.com/v1/audio/speech",
					AuthType: registry.AuthAPIKey, AuthHeader: "bearer", Format: "nvidia-tts",
				},
			},
		},
		{
			ID: "elevenlabs", Display: registry.Display{Name: "ElevenLabs"}, Priority: 4, Category: "media",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.elevenlabs.io/v1/text-to-speech",
					Format:  "elevenlabs", DefaultModel: "eleven_multilingual_v2",
				},
			},
		},
		{
			ID: "selfhosted", Display: registry.Display{Name: "Self Hosted"}, Priority: 3, Category: "media",
			Media: registry.MediaConfigs{
				registry.MediaImage: {Format: "openai", DefaultModel: "sd-xl"},
			},
		},
		{
			ID: "brave-search", Display: registry.Display{Name: "Brave"}, Priority: 4, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaWebSearch: {
					BaseURL: "https://api.search.brave.com/res/v1", Method: "GET",
					QueryParam: "q", MaxResultsParam: "count",
					DefaultMaxResults: 5, MaxMaxResults: 20, CostPerQuery: 0.005,
				},
			},
		},
		{
			// A provider whose own model ids carry a slash, which is why the
			// speech route does not read a voice out of the model string.
			ID: "openrouter", Display: registry.Display{Name: "OpenRouter"}, Priority: 5, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://openrouter.ai/api/v1/chat/completions", Format: "openai",
					DefaultModel: "openai/gpt-4o-mini-tts",
				},
			},
		},
	}
}
