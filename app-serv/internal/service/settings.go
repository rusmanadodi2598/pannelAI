// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/settings.go
// @for       Reading the typed settings document and applying a validated
//
//	partial PATCH to it.
//
// @uses      internal/domain, internal/repository, encoding/json, context, time.
// @reason    SPEC-API-001 §7.14 merges the documented defaults at read and
//
//	validates a PATCH per key. The merge and the per-key persistence
//	live here so a stored row that predates a key still answers with
//	the documented value, and so the deprecated caveman key is carried
//	through untouched rather than dropped by a rewrite.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// SettingsService implements SPEC-API-001 §7.14.
type SettingsService struct {
	repo repository.SettingsRepository
}

// SettingsServiceDeps holds the collaborators the service needs.
type SettingsServiceDeps struct {
	Repo repository.SettingsRepository
}

// NewSettingsService validates deps and returns a ready service.
func NewSettingsService(deps SettingsServiceDeps) (*SettingsService, error) {
	if deps.Repo == nil {
		return nil, domain.NewValidationError("settings repository is required")
	}
	return &SettingsService{repo: deps.Repo}, nil
}

// Settings returns the typed document: the stored values merged over the
// documented defaults, so a deployment that never wrote a setting still answers
// with the §7.14 values.
//
// A stored group that does not decode is skipped rather than failing the read,
// which keeps a hand-edited or newer-version row from locking the panel out of
// its own settings.
func (s *SettingsService) Settings(ctx context.Context) (domain.Settings, error) {
	stored, err := s.repo.Load(ctx)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("loading settings: %w", err)
	}
	settings := domain.DefaultSettings()
	settings.Security = decodeOr(settings.Security, stored[domain.SettingsKeySecurity])
	settings.Routing = decodeOr(settings.Routing, stored[domain.SettingsKeyRouting])
	settings.Network = decodeOr(settings.Network, stored[domain.SettingsKeyNetwork])
	settings.TokenSaver = decodeOr(settings.TokenSaver, stored[domain.SettingsKeyTokenSaver])
	settings.Logging = decodeOr(settings.Logging, stored[domain.SettingsKeyLogging])
	return settings, nil
}

// RequireAPIKey reports whether the data plane must authenticate a request with
// a gateway key (§4: enforced when settings.security.require_api_key is true,
// which is the default). It is the one settings read another vertical needs, so
// it is exposed as a named method rather than making every caller decode the
// document.
func (s *SettingsService) RequireAPIKey(ctx context.Context) (bool, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return true, err
	}
	return settings.Security.RequireAPIKey, nil
}

// Update applies a partial patch, persists only the groups the patch touched,
// and returns the resulting document.
//
// Only touched groups are written, so two concurrent PATCHes of different
// groups cannot clobber each other: each writes its own row. The patch is
// validated against the merged document, so a change that is valid per key but
// incoherent with the stored values is rejected before anything is written.
func (s *SettingsService) Update(ctx context.Context, patch domain.SettingsPatch) (domain.Settings, error) {
	current, err := s.Settings(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	next := current
	if err := next.Update(patch); err != nil {
		return domain.Settings{}, err
	}

	touched := touchedGroups(patch)
	for _, group := range touched {
		payload, err := json.Marshal(groupValue(next, group))
		if err != nil {
			return domain.Settings{}, fmt.Errorf("encoding settings group %s: %w", group, err)
		}
		if err := s.repo.Save(ctx, group, string(payload)); err != nil {
			return domain.Settings{}, fmt.Errorf("storing settings group %s: %w", group, err)
		}
	}
	return next, nil
}

// touchedGroups names the stored keys a patch changes, so an untouched group is
// never rewritten and the deprecated caveman key is never touched at all.
func touchedGroups(patch domain.SettingsPatch) []domain.SettingsKey {
	groups := make([]domain.SettingsKey, 0, 5)
	if patch.Security != nil {
		groups = append(groups, domain.SettingsKeySecurity)
	}
	if patch.Routing != nil {
		groups = append(groups, domain.SettingsKeyRouting)
	}
	if patch.Network != nil {
		groups = append(groups, domain.SettingsKeyNetwork)
	}
	if patch.TokenSaver != nil {
		groups = append(groups, domain.SettingsKeyTokenSaver)
	}
	if patch.Logging != nil {
		groups = append(groups, domain.SettingsKeyLogging)
	}
	return groups
}

// groupValue reads one group's current value, which is what gets persisted.
func groupValue(settings domain.Settings, group domain.SettingsKey) any {
	switch group {
	case domain.SettingsKeySecurity:
		return settings.Security
	case domain.SettingsKeyRouting:
		return settings.Routing
	case domain.SettingsKeyNetwork:
		return settings.Network
	case domain.SettingsKeyTokenSaver:
		return settings.TokenSaver
	case domain.SettingsKeyLogging:
		return settings.Logging
	default:
		return nil
	}
}

// decodeOr decodes a stored group over the default, returning the default when
// the stored value is absent or does not decode. The stored bytes are the I/O
// boundary for this decode (AGENTS.md §1.4): they arrive typed as json.RawMessage
// and never leave the function.
func decodeOr[T any](fallback T, stored string) T {
	if stored == "" {
		return fallback
	}
	var decoded T
	if err := json.Unmarshal([]byte(stored), &decoded); err != nil {
		return fallback
	}
	return decoded
}
