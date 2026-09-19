// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/token_saver_test.go
// @for       Table-driven tests for the §7.9 read and whole replacement.
// @uses      internal/domain, internal/repository, context, sync, testing.
// @reason    The §7.9 write must reach the same patch machinery as the §7.14
//
//	PATCH: only the token_saver row changes, the deprecated caveman
//	key is untouched, and a rejected document persists nothing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubSettingsStore is an in-memory SettingsRepository that records which keys
// were written, so a test can pin the per-row persistence.
type stubSettingsStore struct {
	mu      sync.Mutex
	rows    map[domain.SettingsKey]string
	written []domain.SettingsKey
}

func newStubSettingsStore() *stubSettingsStore {
	return &stubSettingsStore{rows: map[domain.SettingsKey]string{}}
}

func (r *stubSettingsStore) Load(context.Context) (map[domain.SettingsKey]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[domain.SettingsKey]string, len(r.rows))
	for key, value := range r.rows {
		out[key] = value
	}
	return out, nil
}

func (r *stubSettingsStore) Save(_ context.Context, key domain.SettingsKey, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[key] = value
	r.written = append(r.written, key)
	return nil
}

func newTokenSaverFixture(t *testing.T) (*TokenSaverService, *stubSettingsStore) {
	t.Helper()
	repo := newStubSettingsStore()
	svc, err := NewSettingsService(SettingsServiceDeps{Repo: repo})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}
	saver, err := NewTokenSaverService(TokenSaverServiceDeps{Settings: svc})
	if err != nil {
		t.Fatalf("NewTokenSaverService() error = %v", err)
	}
	return saver, repo
}

// TestTokenSaverService_GetServesTheDefaults documents the fresh-install shape:
// the §7.9 defaults, never a 404, with every saver off and the filter allowlist
// empty.
func TestTokenSaverService_GetServesTheDefaults(t *testing.T) {
	saver, _ := newTokenSaverFixture(t)
	got, err := saver.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := domain.DefaultSettings().TokenSaver
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() = %+v, want the §7.9 defaults %+v", got, want)
	}
}

// TestTokenSaverService_Replace covers the write path: accepted documents are
// stored whole and read back identical; rejected ones persist nothing.
func TestTokenSaverService_Replace(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name       string
		next       domain.TokenSaverSettings
		wantErr    string
		wantSaves  int
		wantStored func(t *testing.T, got domain.TokenSaverSettings)
	}{
		{
			name: "every saver on",
			next: domain.TokenSaverSettings{
				RTK:      domain.TokenSaverRTK{Enabled: true, Filters: []string{"git-diff", "grep"}},
				Headroom: domain.TokenSaverHeadroom{Enabled: true, URL: "http://localhost:8787", CompressUserMessages: true},
				Ponytail: domain.TokenSaverToggle{Enabled: true, Level: "ultra"},
			},
			wantSaves: 1,
			wantStored: func(t *testing.T, got domain.TokenSaverSettings) {
				want := domain.TokenSaverSettings{
					RTK:      domain.TokenSaverRTK{Enabled: true, Filters: []string{"git-diff", "grep"}},
					Headroom: domain.TokenSaverHeadroom{Enabled: true, URL: "http://localhost:8787", CompressUserMessages: true},
					Ponytail: domain.TokenSaverToggle{Enabled: true, Level: "ultra"},
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("stored = %+v, want the replaced document %+v", got, want)
				}
			},
		},
		{
			name: "headroom enabled without a url",
			next: domain.TokenSaverSettings{
				RTK:      domain.TokenSaverRTK{Enabled: false, Filters: []string{}},
				Headroom: domain.TokenSaverHeadroom{Enabled: true},
				Ponytail: domain.TokenSaverToggle{Enabled: false, Level: "full"},
			},
			wantErr:   "token_saver.headroom.url is required when headroom is enabled",
			wantSaves: 0,
		},
		{
			name: "headroom url with a non-http scheme",
			next: domain.TokenSaverSettings{
				RTK:      domain.TokenSaverRTK{Enabled: false, Filters: []string{}},
				Headroom: domain.TokenSaverHeadroom{Enabled: true, URL: "gopher://localhost:8787"},
				Ponytail: domain.TokenSaverToggle{Enabled: false, Level: "full"},
			},
			wantErr:   "token_saver.headroom.url must be an absolute http or https URL",
			wantSaves: 0,
		},
		{
			name: "an unknown filter",
			next: domain.TokenSaverSettings{
				RTK:      domain.TokenSaverRTK{Enabled: true, Filters: []string{"summarize"}},
				Headroom: domain.TokenSaverHeadroom{},
				Ponytail: domain.TokenSaverToggle{Enabled: false, Level: "full"},
			},
			wantErr:   "token_saver.rtk.filters",
			wantSaves: 0,
		},
		{
			name: "an empty ponytail level",
			next: domain.TokenSaverSettings{
				RTK:      domain.TokenSaverRTK{Enabled: false, Filters: []string{}},
				Headroom: domain.TokenSaverHeadroom{},
				Ponytail: domain.TokenSaverToggle{Enabled: false, Level: ""},
			},
			wantErr:   "token_saver",
			wantSaves: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saver, repo := newTokenSaverFixture(t)
			got, err := saver.Replace(ctx, tc.next)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Replace() error = %v, want it to contain %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Replace() error = %v", err)
			}
			if len(repo.written) != tc.wantSaves {
				t.Fatalf("saves = %d (%v), want %d", len(repo.written), repo.written, tc.wantSaves)
			}
			if tc.wantErr != "" {
				return
			}
			tc.wantStored(t, got)
			reread, err := saver.Get(ctx)
			if err != nil {
				t.Fatalf("Get() after Replace() error = %v", err)
			}
			if !reflect.DeepEqual(reread, got) {
				t.Fatalf("Get() after Replace() = %+v, want %+v", reread, got)
			}
		})
	}
}

// TestTokenSaverService_ReplaceLeavesOtherRowsAlone pins the blast radius of
// the §7.9 PUT: the token_saver row is written and the deprecated caveman key
// is never among the writes, whichever document lands.
func TestTokenSaverService_ReplaceLeavesOtherRowsAlone(t *testing.T) {
	ctx := context.Background()
	saver, repo := newTokenSaverFixture(t)
	next := domain.TokenSaverSettings{
		RTK:      domain.TokenSaverRTK{Enabled: true, Filters: []string{"git-diff"}},
		Headroom: domain.TokenSaverHeadroom{Enabled: false, URL: "https://compress.internal:8787"},
		Ponytail: domain.TokenSaverToggle{Enabled: false, Level: "full"},
	}
	if _, err := saver.Replace(ctx, next); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	for _, key := range repo.written {
		if key == domain.SettingsKeyCaveman {
			t.Fatalf("the deprecated caveman key was written: %v", repo.written)
		}
		if key != domain.SettingsKeyTokenSaver {
			t.Fatalf("an unrelated settings group was written: %v", repo.written)
		}
	}
	// The other groups still read as their documented defaults, so the PUT
	// cannot have overwritten them.
	svc, err := NewSettingsService(SettingsServiceDeps{Repo: repo})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}
	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatalf("Settings() error = %v", err)
	}
	if settings.Security != domain.DefaultSettings().Security {
		t.Fatalf("security group changed: %+v", settings.Security)
	}
}

// TestTokenSaverService_ReplaceRoundTripsThroughStorage pins that what the
// store holds is the JSON of the replaced group, not a document the read layer
// has to patch up.
func TestTokenSaverService_ReplaceRoundTripsThroughStorage(t *testing.T) {
	ctx := context.Background()
	saver, repo := newTokenSaverFixture(t)
	next := domain.TokenSaverSettings{
		RTK:      domain.TokenSaverRTK{Enabled: true, Filters: []string{"ls", "tree"}},
		Headroom: domain.TokenSaverHeadroom{URL: "http://localhost:8787"},
		Ponytail: domain.TokenSaverToggle{Enabled: true, Level: "ultra"},
	}
	if _, err := saver.Replace(ctx, next); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	raw, ok := repo.rows[domain.SettingsKeyTokenSaver]
	if !ok {
		t.Fatalf("no token_saver row stored")
	}
	var stored domain.TokenSaverSettings
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("stored row does not decode: %v", err)
	}
	if !reflect.DeepEqual(stored, next) {
		t.Fatalf("stored row = %+v, want %+v", stored, next)
	}
}
