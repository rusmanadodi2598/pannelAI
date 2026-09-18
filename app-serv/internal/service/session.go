// Package service orchestrates app-serv use cases without HTTP concerns.
//
// @file      internal/service/session.go
// @for       Authenticated session verification, revocation, and password change.
// @uses      internal/domain, internal/repository, internal/service/password.go.
// @reason    Session state must be checked at every protected management boundary
//
//	and password updates must verify the current credential atomically.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package service

import (
	"context"
	"fmt"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Authenticate verifies token format, HMAC, and Redis revocation state.
func (s *AuthService) Authenticate(ctx context.Context, token string) error {
	digest, err := domain.ParseSessionToken(token, s.secret)
	if err != nil {
		return domain.ErrSessionInvalid
	}
	active, err := s.sessions.Exists(ctx, digest)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if !active {
		return domain.ErrSessionInvalid
	}
	return nil
}

// Logout revokes the currently authenticated session.
func (s *AuthService) Logout(ctx context.Context, token string) error {
	digest, err := domain.ParseSessionToken(token, s.secret)
	if err != nil {
		return domain.ErrSessionInvalid
	}
	if err := s.sessions.Revoke(ctx, digest); err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	return nil
}

// ChangePassword verifies the session and current password before a CAS update.
func (s *AuthService) ChangePassword(ctx context.Context, token, currentPassword, newPassword string) error {
	if err := s.Authenticate(ctx, token); err != nil {
		return err
	}
	currentHash, err := s.repo.PasswordHash(ctx)
	if err != nil {
		return fmt.Errorf("reading current password: %w", err)
	}
	if currentHash == "" {
		return domain.ErrPasswordNotConfigured
	}
	if err := s.hasher.Compare(currentHash, currentPassword); err != nil {
		return domain.ErrInvalidCredentials
	}
	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hashing new password: %w", err)
	}
	if err := s.repo.ChangePassword(ctx, currentHash, newHash); err != nil {
		return err
	}
	return nil
}
