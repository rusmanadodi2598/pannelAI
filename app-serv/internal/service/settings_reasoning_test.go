// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings_reasoning_test.go
// @for       The reasoning group's persistence and the one settings read the
//
//	data plane's injection makes: the thinking mode resolved per provider.
//
// @uses      context, testing, internal/domain.
// @reason    SPEC-API-001 §7.14 stores one mode per provider and AGENTS.md §2.1
//
//	requires the round-trip proven beside the service: a PATCH must write
//	only its own row, and a read of a document that predates the group
//	must answer an empty map rather than nil, because the panel renders
//	"auto" from it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSettingsService_ThinkingModeResolvesPerProvider pins the read the data
// plane makes: one stored document answers each provider's own mode, and a
// provider with no entry resolves to nothing so the injection leaves the
// client's own reasoning intent alone.
func TestSettingsService_ThinkingModeResolvesPerProvider(t *testing.T) {
	repo := newStubSettingsStore()
	stored := `{"provider_thinking":{"openai":{"mode":"high"},"anthropic":{"mode":"on"}}}`
	if err := repo.Save(context.Background(), domain.SettingsKeyReasoning, stored); err != nil {
		t.Fatalf("seeding the reasoning row: %v", err)
	}
	svc := newSettingsUpdateFixture(t, repo)

	cases := []struct {
		provider string
		want     domain.ThinkingMode
		wantOK   bool
	}{
		{"openai", "high", true},
		{"anthropic", domain.ThinkingOn, true},
		{"groq", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			got, ok, err := svc.ThinkingMode(context.Background(), tc.provider)
			if err != nil {
				t.Fatalf("ThinkingMode(%s) error = %v", tc.provider, err)
			}
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("ThinkingMode(%s) = (%q, %v), want (%q, %v)",
					tc.provider, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestSettingsService_UpdatePersistsOnlyTheReasoningRow pins the per-key
// persistence: the patch writes the reasoning group and nothing else, so two
// concurrent PATCHes of different groups cannot clobber each other.
func TestSettingsService_UpdatePersistsOnlyTheReasoningRow(t *testing.T) {
	repo := newStubSettingsStore()
	svc := newSettingsUpdateFixture(t, repo)

	entries := map[string]domain.ProviderThinking{"openai": {Mode: "low"}}
	got, err := svc.Update(context.Background(), domain.SettingsPatch{
		Reasoning: &domain.ReasoningSettingsPatch{ProviderThinking: &entries},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if mode := got.Reasoning.ProviderThinking["openai"].Mode; mode != "low" {
		t.Fatalf("returned provider_thinking[openai].mode = %q, want low", mode)
	}
	want := []domain.SettingsKey{domain.SettingsKeyReasoning}
	if !reflect.DeepEqual(repo.written, want) {
		t.Fatalf("written keys = %v, want %v", repo.written, want)
	}
	rows, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if stored := rows[domain.SettingsKeyReasoning]; stored != `{"provider_thinking":{"openai":{"mode":"low"}}}` {
		t.Fatalf("stored reasoning row = %s, want the whole map", stored)
	}
}

// TestSettingsService_ThinkingModeNormalizesAPreChangeRow pins the read path
// for a deployment that never wrote the group: the accessor answers false and
// the document answers an empty map, never nil, so the panel's round-trip
// cannot turn "no entries" into a missing member.
func TestSettingsService_ThinkingModeNormalizesAPreChangeRow(t *testing.T) {
	svc := newSettingsUpdateFixture(t, newStubSettingsStore())

	if _, ok, err := svc.ThinkingMode(context.Background(), "openai"); err != nil || ok {
		t.Fatalf("ThinkingMode() = (_, %v, %v), want no entry and no error", ok, err)
	}
	read, err := svc.Settings(context.Background())
	if err != nil {
		t.Fatalf("Settings() error = %v", err)
	}
	if read.Reasoning.ProviderThinking == nil {
		t.Fatal("provider_thinking = nil, want an empty map so the panel's read renders {}")
	}
}

// TestSettingsService_ThinkingModeSurfacesAReadFailure pins the direction the
// service takes: the failure is returned, not hidden, so the injection can tell
// a failed read from an unstored mode.
func TestSettingsService_ThinkingModeSurfacesAReadFailure(t *testing.T) {
	svc, err := NewSettingsService(SettingsServiceDeps{Repo: failingSettingsStore{}})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}

	if _, _, err := svc.ThinkingMode(context.Background(), "openai"); err == nil {
		t.Fatal("ThinkingMode() error = nil, want the read failure")
	}
}
