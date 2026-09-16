// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/gateway_key.go
// @for       The gateway key lifecycle: issue, list, inspect, rename, revoke.
// @uses      internal/domain, internal/repository.
// @reason    SPEC-API-001 §7.3 fixes this behavior (plaintext returned once,
//
//	hints thereafter, terminal revocation) and AGENTS.md §1.5 puts
//	orchestration in this layer with no net/http import.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     service
// @stability experimental
// @since     2026-09-16
package service

import (
	"context"
	"crypto/rand"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// keyAlphabet omits ambiguous characters so a copied key stays unambiguous.
const keyAlphabet = "abcdefghijkmnpqrstuvwxyz23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// keyRandomLen is the body length of a gateway key, after the prefix.
const keyRandomLen = 48

// GatewayKeyService implements SPEC-API-001 §7.3.
type GatewayKeyService struct {
	repo   repository.GatewayKeyRepository
	prefix string
	clock  func() time.Time
}

// GatewayKeyServiceDeps holds the collaborators the service needs.
type GatewayKeyServiceDeps struct {
	Repo   repository.GatewayKeyRepository
	Prefix string
}

// NewGatewayKeyService validates deps and returns a ready service.
func NewGatewayKeyService(deps GatewayKeyServiceDeps) (*GatewayKeyService, error) {
	if deps.Repo == nil {
		return nil, domain.NewValidationError("gateway key repository is required")
	}
	if deps.Prefix == "" {
		return nil, domain.NewValidationError("gateway key prefix is required")
	}
	return &GatewayKeyService{
		repo:   deps.Repo,
		prefix: deps.Prefix,
		clock:  time.Now,
	}, nil
}

// Create mints a new key. The plaintext is returned to the caller once; the
// repository stores only the digest and the masking hint (SPEC-API-001 §4).
func (s *GatewayKeyService) Create(ctx context.Context, name string) (domain.GatewayKey, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.GatewayKey{}, "", domain.NewValidationError("name is required")
	}
	if len(name) > 120 {
		return domain.GatewayKey{}, "", domain.NewValidationError("name must be at most 120 characters")
	}

	plaintext, err := s.generate()
	if err != nil {
		return domain.GatewayKey{}, "", domain.NewInternalError("could not generate gateway key")
	}

	key := domain.NewGatewayKey(name, plaintext, MaskGatewayKey(s.prefix, plaintext), s.clock())
	if err := s.repo.Create(ctx, key); err != nil {
		return domain.GatewayKey{}, "", err
	}
	return key, plaintext, nil
}

// List returns one page of keys, hints only.
func (s *GatewayKeyService) List(ctx context.Context, page, perPage int) ([]domain.GatewayKey, int64, error) {
	return s.repo.List(ctx, repository.PageQuery{Page: page, PerPage: perPage})
}

// Get returns a single key by ID, without its secret.
func (s *GatewayKeyService) Get(ctx context.Context, id string) (domain.GatewayKey, error) {
	return s.repo.GetByID(ctx, id)
}

// Update applies a partial PATCH. Nil pointers mean "leave unchanged" (§7.3).
func (s *GatewayKeyService) Update(ctx context.Context, id string, name *string, status *string) (domain.GatewayKey, error) {
	key, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.GatewayKey{}, err
	}

	if name != nil {
		if err := key.Rename(strings.TrimSpace(*name)); err != nil {
			return domain.GatewayKey{}, err
		}
	}
	if status != nil {
		next, err := domain.ParseGatewayKeyStatus(*status)
		if err != nil {
			return domain.GatewayKey{}, err
		}
		if err := key.Transition(next); err != nil {
			return domain.GatewayKey{}, err
		}
	}

	if err := s.repo.Update(ctx, key); err != nil {
		return domain.GatewayKey{}, err
	}
	return key, nil
}

// Revoke applies a terminal revocation (§7.3 DELETE).
func (s *GatewayKeyService) Revoke(ctx context.Context, id string) error {
	key, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := key.Revoke(s.clock()); err != nil {
		return err
	}
	return s.repo.Revoke(ctx, id, *key.RevokedAt())
}

// generate returns prefix + keyRandomLen unbiased random characters.
func (s *GatewayKeyService) generate() (string, error) {
	buf := make([]byte, keyRandomLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 0, len(s.prefix)+keyRandomLen)
	out = append(out, s.prefix...)

	// Rejection sampling: only bytes below the largest multiple of the alphabet
	// length are used, so every character is equally likely. Taking byte%len
	// directly would favour the first 256%len characters and shrink the key's
	// effective entropy.
	limit := byte(256 - (256 % len(keyAlphabet)))
	for _, b := range buf {
		if b < limit {
			out = append(out, keyAlphabet[int(b)%len(keyAlphabet)])
		}
	}

	// Read more entropy until the requested length is reached; the loop above
	// discards at most 256%len of 256 values, so this converges immediately.
	for len(out) < len(s.prefix)+keyRandomLen {
		var extra [32]byte
		if _, err := rand.Read(extra[:]); err != nil {
			return "", err
		}
		for _, b := range extra {
			if len(out) >= len(s.prefix)+keyRandomLen {
				break
			}
			if b < limit {
				out = append(out, keyAlphabet[int(b)%len(keyAlphabet)])
			}
		}
	}
	return string(out), nil
}
