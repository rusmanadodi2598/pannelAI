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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
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

// GetByValueHash mirrors the real lookup: the stub stores keys by id but must
// answer a digest query, so it scans for the digest the same way the index
// does. Enforcing the contract here keeps the stub from accepting a call the
// real repository would reject.
func (r *memKeyRepo) GetByValueHash(ctx context.Context, valueHash string) (domain.GatewayKey, error) {
	for _, k := range r.db {
		if k.ValueHash() == valueHash {
			return k, nil
		}
	}
	return domain.GatewayKey{}, domain.ErrGatewayKeyNotFound
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

// RecordUse mirrors the real counter update, so a router test can assert what
// an admitted data-plane call does to the key that presented the credential.
func (r *memKeyRepo) RecordUse(ctx context.Context, id string, usedAt time.Time) error {
	k, ok := r.db[id]
	if !ok {
		return domain.ErrGatewayKeyNotFound
	}
	stamp := usedAt
	r.db[id] = domain.RehydrateGatewayKey(k.ID(), k.Name(), k.ValueHash(), k.KeyHint(),
		k.Status(), &stamp, k.RequestCount()+1, k.CreatedAt(), k.RevokedAt())
	return nil
}

// newTestRouter builds the real authenticated mux and injects a valid session
// cookie into legacy CRUD tests; auth-specific tests exercise the guard without
// that fixture.
func newTestRouter(t *testing.T) *Mux {
	t.Helper()
	mux, _ := newAuthenticatedRouter(t)
	login := httptest.NewRecorder()
	mux.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`)))
	if login.Code != http.StatusNoContent {
		t.Fatalf("test auth login: %d (%s)", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	original := mux.Handler
	mux.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		original.ServeHTTP(w, r)
	})
	return mux
}
