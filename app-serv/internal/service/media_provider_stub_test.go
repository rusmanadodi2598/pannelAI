// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_provider_stub_test.go
// @for       The in-memory override store, media-capable index, and endpoint
//
//	counter the §7.10 tests drive.
//
// @uses      context, internal/domain, internal/registry, testing.
// @reason    The three collaborators are the ones the service must not own, so
//
//	all three are doubles here; keeping them together leaves each test
//	about the rule it pins.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"sort"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stubMediaRepo is an in-memory MediaOverrideRepository keyed by provider and
// kind, mirroring the primary key the migration declares.
type stubMediaRepo struct {
	rows    map[string]domain.MediaOverride
	upserts int
	failure error
}

func newStubMediaRepo() *stubMediaRepo {
	return &stubMediaRepo{rows: map[string]domain.MediaOverride{}}
}

func (r *stubMediaRepo) List(_ context.Context) ([]domain.MediaOverride, error) {
	if r.failure != nil {
		return nil, r.failure
	}
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

func (r *stubMediaRepo) Get(_ context.Context, providerID string, kind domain.MediaKind) (domain.MediaOverride, bool, error) {
	if r.failure != nil {
		return domain.MediaOverride{}, false, r.failure
	}
	row, ok := r.rows[overrideKey(providerID, kind)]
	return row, ok, nil
}

func (r *stubMediaRepo) Upsert(_ context.Context, override domain.MediaOverride) error {
	if r.failure != nil {
		return r.failure
	}
	r.upserts++
	r.rows[overrideKey(override.ProviderID(), override.Kind())] = override
	return nil
}

// mediaIndex is a fixed ProviderIndex over the entries a case declares.
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

// stubMediaCounts answers the endpoint roll-up for the ids it was given.
type stubMediaCounts struct {
	byProvider map[string]domain.EndpointStatusCounts
}

func (c stubMediaCounts) EndpointStatusCountsByProvider(_ context.Context, ids []string) (map[string]domain.EndpointStatusCounts, error) {
	out := make(map[string]domain.EndpointStatusCounts, len(ids))
	for _, id := range ids {
		out[id] = c.byProvider[id]
	}
	return out, nil
}

// mediaFixture wires the service over the doubles and the given entries. A nil
// counts double leaves the endpoint count at zero, which is the optional case.
func mediaFixture(t *testing.T, entries []registry.Provider, counts EndpointCounterByProvider) (*MediaProviderService, *stubMediaRepo) {
	t.Helper()
	repo := newStubMediaRepo()
	svc, err := NewMediaProviderService(MediaProviderServiceDeps{
		Index: &mediaIndex{entries: entries}, Repo: repo, Counts: counts,
	})
	if err != nil {
		t.Fatalf("NewMediaProviderService() error = %v", err)
	}
	return svc, repo
}

// mediaProviderEntries is the fixed registry the tests read: one provider with
// three kinds, one with a single kind, one hidden, and one with no media at all.
func mediaProviderEntries() []registry.Provider {
	return []registry.Provider{
		{
			ID: "openai", Display: registry.Display{Name: "OpenAI"}, Priority: 1, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.openai.com/v1", DefaultModel: "tts-1",
					Models: []registry.MediaModel{{ID: "tts-1", Name: "TTS 1"}, {ID: "tts-2", Name: "TTS 2"}},
				},
				registry.MediaImage: {
					BaseURL: "https://api.openai.com/v1", DefaultModel: "dall-e-3",
					Models: []registry.MediaModel{{ID: "dall-e-3", Name: "DALL-E 3"}},
				},
				registry.MediaEmbedding: {
					BaseURL: "https://api.openai.com/v1", DefaultModel: "text-embedding-3-small",
				},
			},
		},
		{
			ID: "elevenlabs", Display: registry.Display{Name: "ElevenLabs"}, Priority: 2, Category: "media",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.elevenlabs.io/v1",
					Models:  []registry.MediaModel{{ID: "eleven_multilingual_v2", Name: "Multilingual v2"}},
				},
			},
		},
		{
			ID: "hidden-media", Display: registry.Display{Name: "Hidden"}, Priority: 3, Hidden: true,
			Media: registry.MediaConfigs{
				registry.MediaTTS: {BaseURL: "https://hidden.example.com"},
			},
		},
		{ID: "chatonly", Display: registry.Display{Name: "Chat Only"}, Priority: 4, Category: "apikey"},
	}
}
