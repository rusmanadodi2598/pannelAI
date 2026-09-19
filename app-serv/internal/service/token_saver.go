// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/token_saver.go
// @for       Reading and wholly replacing the §7.9 token-saver configuration.
// @uses      internal/domain, internal/repository, context.
// @reason    SPEC-API-001 §7.9 gives the saver configuration its own endpoints
//
//	so the panel edits it without round-tripping the whole §7.14
//	document. The write lands in the same settings patch machinery the
//	§7.14 PATCH uses, so there is exactly one persistence path for the
//	group and the deprecated caveman key is untouched by construction.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TokenSaverService implements SPEC-API-001 §7.9.
type TokenSaverService struct {
	settings *SettingsService
}

// TokenSaverServiceDeps holds the collaborators the service needs.
type TokenSaverServiceDeps struct {
	Settings *SettingsService
}

// NewTokenSaverService validates deps and returns a ready service.
func NewTokenSaverService(deps TokenSaverServiceDeps) (*TokenSaverService, error) {
	if deps.Settings == nil {
		return nil, domain.NewValidationError("settings service is required")
	}
	return &TokenSaverService{settings: deps.Settings}, nil
}

// Get returns the §7.9 configuration: the stored token_saver group merged over
// the documented defaults, exactly as the §7.14 document renders it.
func (s *TokenSaverService) Get(ctx context.Context) (domain.TokenSaverSettings, error) {
	settings, err := s.settings.Settings(ctx)
	if err != nil {
		return domain.TokenSaverSettings{}, err
	}
	return settings.TokenSaver, nil
}

// Replace stores the document as the new token_saver group and returns what was
// stored. Every field of the group is written, so a client that PUTs back what
// it GET never leaves a stale field behind.
func (s *TokenSaverService) Replace(ctx context.Context, next domain.TokenSaverSettings) (domain.TokenSaverSettings, error) {
	patch := domain.SettingsPatch{TokenSaver: &domain.TokenSaverSettingsPatch{
		RTK:      replaceRTK(next.RTK),
		Headroom: replaceHeadroom(next.Headroom),
		Ponytail: replaceToggle(next.Ponytail),
	}}
	settings, err := s.settings.Update(ctx, patch)
	if err != nil {
		return domain.TokenSaverSettings{}, err
	}
	return settings.TokenSaver, nil
}

// replaceRTK turns the native engine's group into a patch that writes every
// field, the filter allowlist included.
func replaceRTK(r domain.TokenSaverRTK) *domain.TokenSaverRTKPatch {
	filters := append([]string{}, r.Filters...)
	return &domain.TokenSaverRTKPatch{Enabled: &r.Enabled, Filters: &filters}
}

// replaceToggle turns one saver group into a patch that writes every field.
func replaceToggle(t domain.TokenSaverToggle) *domain.TokenSaverTogglePatch {
	return &domain.TokenSaverTogglePatch{Enabled: &t.Enabled, Level: &t.Level}
}

// replaceHeadroom turns the compression group into a patch that writes every field.
func replaceHeadroom(h domain.TokenSaverHeadroom) *domain.TokenSaverHeadroomPatch {
	return &domain.TokenSaverHeadroomPatch{Enabled: &h.Enabled, URL: &h.URL, CompressUserMessages: &h.CompressUserMessages}
}
