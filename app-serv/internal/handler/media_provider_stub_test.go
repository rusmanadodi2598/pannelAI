// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_provider_stub_test.go
// @for       The in-memory override store, media index, and handler fixture the
//
//	§7.10 route tests drive.
//
// @uses      internal/domain, internal/registry, internal/service, context,
//
//	sort, testing.
//
// @reason    The handler takes a concrete *service.MediaProviderService, so the
//
//	route tests build the real service over doubles rather than faking the
//	service itself — the seam the production wiring uses is the same one
//	the tests use.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"context"
	"net/http"
	"sort"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// withPathValue installs the named path value a Go 1.22 ServeMux would
// populate. The handler tests call methods directly, so they must set it.
func withPathValue(fn http.HandlerFunc, name, value string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue(name, value)
		fn(w, r)
	}
}

// stubMediaOverrideRepo is an in-memory MediaOverrideRepository.
type stubMediaOverrideRepo struct {
	rows map[string]domain.MediaOverride
}

func newStubMediaOverrideRepo() *stubMediaOverrideRepo {
	return &stubMediaOverrideRepo{rows: map[string]domain.MediaOverride{}}
}

func (r *stubMediaOverrideRepo) List(context.Context) ([]domain.MediaOverride, error) {
	out := make([]domain.MediaOverride, 0, len(r.rows))
	for _, row := range r.rows {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderID() != out[j].ProviderID() {
			return out[i].ProviderID() < out[j].ProviderID()
		}
		return out[i].Kind() < out[j].Kind()
	})
	return out, nil
}

func (r *stubMediaOverrideRepo) Get(_ context.Context, providerID string, kind domain.MediaKind) (domain.MediaOverride, bool, error) {
	row, ok := r.rows[providerID+"\x00"+string(kind)]
	return row, ok, nil
}

func (r *stubMediaOverrideRepo) Upsert(_ context.Context, override domain.MediaOverride) error {
	r.rows[override.ProviderID()+"\x00"+string(override.Kind())] = override
	return nil
}

// mediaIndex is a fixed ProviderIndex over the entries the fixture declares.
type mediaIndex struct {
	entries []registry.Provider
}

func (i *mediaIndex) Provider(name string) (registry.Provider, bool) {
	for _, entry := range i.entries {
		if entry.ID == name {
			return entry, true
		}
	}
	return registry.Provider{}, false
}

func (i *mediaIndex) All() []registry.Provider { return i.entries }

func (i *mediaIndex) Categories() []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, entry := range i.entries {
		if entry.Category != "" && !seen[entry.Category] {
			seen[entry.Category] = true
			out = append(out, entry.Category)
		}
	}
	sort.Strings(out)
	return out
}

// newMediaProviderFixture wires the §7.10 service over the doubles. The index
// holds one provider with two kinds and one that offers none, so a case can
// exercise both the populated and the empty answer.
func newMediaProviderFixture(t *testing.T) (*MediaProviderHandler, *stubMediaOverrideRepo) {
	t.Helper()
	repo := newStubMediaOverrideRepo()
	svc, err := service.NewMediaProviderService(service.MediaProviderServiceDeps{
		Index: &mediaIndex{entries: []registry.Provider{
			{
				ID: "openai", Display: registry.Display{Name: "OpenAI"}, Priority: 1, Category: "apikey",
				Media: registry.MediaConfigs{
					registry.MediaTTS: {
						BaseURL: "https://api.openai.com/v1", DefaultModel: "tts-1",
						Models: []registry.MediaModel{{ID: "tts-1", Name: "TTS 1"}},
					},
					registry.MediaImage: {
						BaseURL: "https://api.openai.com/v1", DefaultModel: "dall-e-3",
					},
				},
			},
			{ID: "chatonly", Display: registry.Display{Name: "Chat Only"}, Priority: 2},
		}},
		Repo: repo,
	})
	if err != nil {
		t.Fatalf("NewMediaProviderService() error = %v", err)
	}
	return NewMediaProviderHandler(svc), repo
}
