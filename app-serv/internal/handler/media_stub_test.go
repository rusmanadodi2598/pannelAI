// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_stub_test.go
// @for       The media caller, router, index, and authenticator the §7.10
//
//	data-plane route tests drive.
//
// @uses      internal/dataplane, internal/domain, internal/registry,
//
//	internal/service, context, testing, time.
//
// @reason    The handler takes a concrete *service.MediaCallService, so these
//
//	build the real service over doubles rather than faking the service —
//	the seam production uses is the seam the tests use.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// stubMediaCaller records the outbound request and answers with a canned
// response.
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

// stubMediaRouter answers selection with one fixed endpoint.
type stubMediaRouter struct{}

func (stubMediaRouter) Select(_ context.Context, providerID string) (dataplane.Selection, error) {
	endpoint, err := domain.NewUpstreamEndpoint("ep-"+providerID, providerID, "primary",
		domain.UpstreamAuthAPIKey, 1, time.Now().UTC())
	if err != nil {
		return dataplane.Selection{}, err
	}
	return dataplane.Selection{Endpoint: endpoint}, nil
}

func (stubMediaRouter) RecordSuccess(context.Context, dataplane.Selection) error { return nil }
func (stubMediaRouter) RecordFailure(context.Context, dataplane.Selection, string, domain.KeyFailureClass) error {
	return nil
}

// stubAuthenticator is the §4 seam: it accepts one token and refuses the rest,
// so a route test can prove both the accepted and the refused path without the
// engine the chat service would need.
type stubAuthenticator struct {
	token  string
	refuse error
}

func (a stubAuthenticator) Authenticate(_ context.Context, presented string) (domain.GatewayKey, error) {
	if a.refuse != nil {
		return domain.GatewayKey{}, a.refuse
	}
	if a.token == "" || presented == a.token {
		return domain.GatewayKey{}, nil
	}
	return domain.GatewayKey{}, dataplane.ErrUnauthorized()
}

// newMediaHandlerFixture wires the §7.10 data-plane handler over the doubles.
// The index holds one OpenAI-shaped provider with speech, transcription, and
// image blocks, and one search provider.
func newMediaHandlerFixture(t *testing.T) (*MediaHandler, *stubMediaCaller) {
	t.Helper()
	caller := &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}}
	svc, err := service.NewMediaCallService(service.MediaCallServiceDeps{
		Index:  &handlerMediaIndex{entries: handlerMediaEntries()},
		Router: stubMediaRouter{},
		Caller: caller,
	})
	if err != nil {
		t.Fatalf("NewMediaCallService() error = %v", err)
	}
	return NewMediaHandler(svc, stubAuthenticator{}), caller
}

// handlerMediaEntries is the registry the data-plane route tests read.
func handlerMediaEntries() []registry.Provider {
	return []registry.Provider{
		{
			ID: "openai", Display: registry.Display{Name: "OpenAI"}, Priority: 1, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.openai.com/v1/audio/speech", Format: "openai",
					DefaultModel: "gpt-4o-mini-tts",
					Voices:       []registry.MediaVoice{{ID: "alloy", Name: "Alloy"}},
				},
				registry.MediaSTT: {
					BaseURL: "https://api.openai.com/v1/audio/transcriptions", Format: "openai",
					DefaultModel: "whisper-1",
				},
				registry.MediaImage: {
					BaseURL: "https://api.openai.com/v1/images/generations", Format: "openai",
					DefaultModel: "gpt-image-1",
				},
			},
		},
		{
			ID: "brave-search", Display: registry.Display{Name: "Brave"}, Priority: 2, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaWebSearch: {
					BaseURL: "https://api.search.brave.com/res/v1", Method: "GET",
					QueryParam: "q", MaxResultsParam: "count", DefaultMaxResults: 5,
				},
			},
		},
	}
}

// handlerMediaIndex is a fixed ProviderIndex over the entries a test declares.
type handlerMediaIndex struct {
	entries []registry.Provider
}

func (i *handlerMediaIndex) Provider(name string) (registry.Provider, bool) {
	for _, entry := range i.entries {
		if entry.ID == name {
			return entry, true
		}
	}
	return registry.Provider{}, false
}

func (i *handlerMediaIndex) All() []registry.Provider { return i.entries }

func (i *handlerMediaIndex) Categories() []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, entry := range i.entries {
		if entry.Category != "" && !seen[entry.Category] {
			seen[entry.Category] = true
			out = append(out, entry.Category)
		}
	}
	return out
}
