// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/key_revocation_test.go
// @for       The §7.3 revocation journey: a DELETE'd key stops serving the
//
//	data plane, indistinguishably from a key that never existed.
//
// @uses      internal/domain, internal/repository, context, testing, time.
// @reason    The revocation guarantee lives in the lookup's "active,
//
//	non-revoked" rule (chat.go's GatewayKeyLookup contract and the
//	Postgres WHERE clause). The auth tests stub that lookup, so nothing
//	else pins the journey an operator relies on: issue a key, revoke it
//	through §7.3, and the same plaintext is refused at the data plane.
//	The in-memory repo mirrors the SQL semantics so the test fails if
//	either side drifts.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// memGatewayKeyRepo is an in-memory GatewayKeyRepository whose hash lookup
// carries the SQL rule it replaces: digest equality, status active, and no
// revocation instant. A fake that answered every hash would make the journey
// test pass while the real database refuses nothing.
type memGatewayKeyRepo struct {
	keys map[string]domain.GatewayKey
}

func newMemGatewayKeyRepo() *memGatewayKeyRepo {
	return &memGatewayKeyRepo{keys: map[string]domain.GatewayKey{}}
}

func (r *memGatewayKeyRepo) Create(_ context.Context, key domain.GatewayKey) error {
	r.keys[key.ValueHash()] = key
	return nil
}

func (r *memGatewayKeyRepo) List(_ context.Context, _ repository.PageQuery) ([]domain.GatewayKey, int64, error) {
	out := make([]domain.GatewayKey, 0, len(r.keys))
	for _, key := range r.keys {
		out = append(out, key)
	}
	return out, int64(len(out)), nil
}

func (r *memGatewayKeyRepo) GetByID(_ context.Context, id string) (domain.GatewayKey, error) {
	for _, key := range r.keys {
		if key.ID() == id {
			return key, nil
		}
	}
	return domain.GatewayKey{}, domain.ErrGatewayKeyNotFound
}

func (r *memGatewayKeyRepo) GetByValueHash(_ context.Context, valueHash string) (domain.GatewayKey, error) {
	key, ok := r.keys[valueHash]
	if !ok || key.Status() != domain.GatewayKeyActive || key.RevokedAt() != nil {
		return domain.GatewayKey{}, domain.ErrGatewayKeyNotFound
	}
	return key, nil
}

func (r *memGatewayKeyRepo) Update(_ context.Context, key domain.GatewayKey) error {
	r.keys[key.ValueHash()] = key
	return nil
}

func (r *memGatewayKeyRepo) Revoke(_ context.Context, id string, revokedAt time.Time) error {
	for hash, key := range r.keys {
		if key.ID() == id {
			key.Revoke(revokedAt)
			r.keys[hash] = key
			return nil
		}
	}
	return domain.ErrGatewayKeyNotFound
}

func (r *memGatewayKeyRepo) RecordUse(_ context.Context, _ string, _ time.Time) error { return nil }

// TestRevokedKeyStopsServingTheDataPlane walks the operator's journey: a key is
// issued through §7.3, it admits a data-plane call, DELETE revokes it, and the
// same plaintext is then refused exactly like an unknown key, so a client
// cannot probe whether a key ever existed.
func TestRevokedKeyStopsServingTheDataPlane(t *testing.T) {
	repo := newMemGatewayKeyRepo()
	keys, err := NewGatewayKeyService(GatewayKeyServiceDeps{Repo: repo, Prefix: "sk-"})
	if err != nil {
		t.Fatalf("gateway key service: %v", err)
	}
	chat := &ChatService{
		keys:     repo,
		settings: stubRequireKey{required: true},
		clock:    time.Now,
	}
	ctx := context.Background()

	key, plaintext, err := keys.Create(ctx, "panel")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := chat.Authenticate(ctx, plaintext); err != nil {
		t.Fatalf("Authenticate() refused a live key: %v", err)
	}

	if err := keys.Revoke(ctx, key.ID()); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	_, err = chat.Authenticate(ctx, plaintext)
	if err == nil {
		t.Fatal("Authenticate() admitted a revoked key")
	}
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != "UNAUTHORIZED" {
		t.Fatalf("Authenticate() error = %v, want the UNAUTHORIZED code", err)
	}

	if _, err := chat.Authenticate(ctx, "sk-never-issued"); err == nil {
		t.Fatal("Authenticate() admitted a key that never existed")
	}
}
