// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_stub_test.go
// @for       In-memory GatewayKeyRepository stub and the router test harness.
// @uses      internal/domain, internal/handler, internal/repository,
//
//	internal/router, internal/schema, internal/service, net/http/httptest.
//
// @reason    AGENTS.md §2.1 requires route tests to run against the real mux;
//
//	this stub implements the repository contract so those tests need no
//	live database, and it enforces the documented uniqueness rule
//	instead of accepting what the real constraint would reject.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// memKeyRepo is an in-memory GatewayKeyRepository for route tests. Keying
// duplicates by name mirrors the contract the repository interface states and
// the UNIQUE index enforces; keying by id would accept every duplicate, because
// a freshly minted ULID is always new.
type memKeyRepo struct {
	db   map[string]domain.GatewayKey
	name map[string]string
}

func newMemKeyRepo() *memKeyRepo {
	return &memKeyRepo{db: map[string]domain.GatewayKey{}, name: map[string]string{}}
}

func (r *memKeyRepo) Create(ctx context.Context, key domain.GatewayKey) error {
	if _, dup := r.name[key.Name()]; dup {
		return domain.ErrGatewayKeyExists
	}
	r.db[key.ID()] = key
	r.name[key.Name()] = key.ID()
	return nil
}

func (r *memKeyRepo) List(ctx context.Context, q repository.PageQuery) ([]domain.GatewayKey, int64, error) {
	all := make([]domain.GatewayKey, 0, len(r.db))
	for _, k := range r.db {
		all = append(all, k)
	}
	start := q.Offset()
	if start >= len(all) {
		return []domain.GatewayKey{}, int64(len(all)), nil
	}
	end := start + q.PerPage
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], int64(len(all)), nil
}

func (r *memKeyRepo) GetByID(ctx context.Context, id string) (domain.GatewayKey, error) {
	k, ok := r.db[id]
	if !ok {
		return domain.GatewayKey{}, domain.ErrGatewayKeyNotFound
	}
	return k, nil
}

func (r *memKeyRepo) Update(ctx context.Context, key domain.GatewayKey) error {
	existing, ok := r.db[key.ID()]
	if !ok {
		return domain.ErrGatewayKeyNotFound
	}
	// A rename onto another key's name collides with the UNIQUE index in the
	// real schema; enforcing it here keeps the stub from accepting what
	// PostgreSQL would reject.
	if owner, taken := r.name[key.Name()]; taken && owner != key.ID() {
		return domain.ErrGatewayKeyExists
	}
	delete(r.name, existing.Name())
	r.db[key.ID()] = key
	r.name[key.Name()] = key.ID()
	return nil
}

func (r *memKeyRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	k, ok := r.db[id]
	if !ok {
		return domain.ErrGatewayKeyNotFound
	}
	r.db[id] = domain.RehydrateGatewayKey(k.ID(), k.Name(), k.ValueHash(), k.KeyHint(),
		domain.GatewayKeyRevoked, k.LastUsedAt(), k.RequestCount(), k.CreatedAt(), &revokedAt)
	return nil
}

// newTestRouter builds the real mux over an in-memory repository.
func newTestRouter(t *testing.T) *Mux {
	t.Helper()
	svc, err := service.NewGatewayKeyService(service.GatewayKeyServiceDeps{
		Repo: newMemKeyRepo(), Prefix: "sk-",
	})
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		GatewayKey: handler.NewGatewayKeyHandler(svc),
	})
}
