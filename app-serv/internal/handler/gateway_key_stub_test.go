// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/gateway_key_stub_test.go
// @for       In-memory GatewayKeyRepository stub and the HTTP test helpers.
// @uses      internal/domain, internal/repository, internal/service,
//
//	internal/handler, net/http/httptest.
//
// @reason    The HTTP tests need the real repository contract without a live
//
//	database; this stub implements the same interface the postgres
//	repository does, so the service and handler are exercised through
//	the contract rather than through mocks of their own output.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-16
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// stubKeyRepo is an in-memory GatewayKeyRepository. It keeps tests free of a
// live PostgreSQL while still exercising real service behavior through the
// repository contract.
type stubKeyRepo struct {
	mu   sync.Mutex
	db   map[string]domain.GatewayKey
	name map[string]string
}

func newStubKeyRepo() *stubKeyRepo {
	return &stubKeyRepo{db: map[string]domain.GatewayKey{}, name: map[string]string{}}
}

func (r *stubKeyRepo) Create(ctx context.Context, key domain.GatewayKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.name[key.Name()]; dup {
		return domain.ErrGatewayKeyExists
	}
	r.db[key.ID()] = key
	r.name[key.Name()] = key.ID()
	return nil
}

func (r *stubKeyRepo) List(ctx context.Context, q repository.PageQuery) ([]domain.GatewayKey, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]domain.GatewayKey, 0, len(r.db))
	for _, k := range r.db {
		all = append(all, k)
	}
	total := int64(len(all))

	start := q.Offset()
	if start >= len(all) {
		return []domain.GatewayKey{}, total, nil
	}
	end := start + q.PerPage
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], total, nil
}

func (r *stubKeyRepo) GetByID(ctx context.Context, id string) (domain.GatewayKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.db[id]
	if !ok {
		return domain.GatewayKey{}, domain.ErrGatewayKeyNotFound
	}
	return k, nil
}

// GetByValueHash answers a digest lookup the way the real repository does, so a
// handler test that authenticates a presented token exercises the same path.
func (r *stubKeyRepo) GetByValueHash(ctx context.Context, valueHash string) (domain.GatewayKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range r.db {
		if k.ValueHash() == valueHash {
			return k, nil
		}
	}
	return domain.GatewayKey{}, domain.ErrGatewayKeyNotFound
}

func (r *stubKeyRepo) Update(ctx context.Context, key domain.GatewayKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.db[key.ID()]; !ok {
		return domain.ErrGatewayKeyNotFound
	}

	// A rename onto another key's name collides with the UNIQUE index in the
	// real schema; enforcing it here keeps the stub from accepting what
	// PostgreSQL would reject.
	if owner, taken := r.name[key.Name()]; taken && owner != key.ID() {
		return domain.ErrGatewayKeyExists
	}
	delete(r.name, r.db[key.ID()].Name())
	r.db[key.ID()] = key
	r.name[key.Name()] = key.ID()
	return nil
}

func (r *stubKeyRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.db[id]
	if !ok {
		return domain.ErrGatewayKeyNotFound
	}
	k = rehydrateRevoked(k, revokedAt)
	r.db[id] = k
	return nil
}

// RecordUse mirrors the real counter update, so a handler test can assert what
// an admitted data-plane call does to the key that presented the credential.
func (r *stubKeyRepo) RecordUse(ctx context.Context, id string, usedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.db[id]
	if !ok {
		return domain.ErrGatewayKeyNotFound
	}
	stamp := usedAt
	r.db[id] = domain.RehydrateGatewayKey(
		k.ID(), k.Name(), k.ValueHash(), k.KeyHint(),
		k.Status(), &stamp, k.RequestCount()+1, k.CreatedAt(), k.RevokedAt())
	return nil
}

func rehydrateRevoked(k domain.GatewayKey, at time.Time) domain.GatewayKey {
	return domain.RehydrateGatewayKey(
		k.ID(), k.Name(), k.ValueHash(), k.KeyHint(),
		domain.GatewayKeyRevoked, k.LastUsedAt(), k.RequestCount(), k.CreatedAt(), &at)
}

// newTestHandler wires the handler against an in-memory repository.
func newTestHandler(t *testing.T) (*GatewayKeyHandler, *stubKeyRepo) {
	t.Helper()
	repo := newStubKeyRepo()
	svc, err := service.NewGatewayKeyService(service.GatewayKeyServiceDeps{Repo: repo, Prefix: "sk-"})
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	return NewGatewayKeyHandler(svc), repo
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decoding response body: %v (body: %s)", err, rr.Body.String())
	}
	return out
}

// setPathID installs the {id} path value a Go 1.22 ServeMux would populate.
// The handler-level tests call methods directly, so they must set it manually.
func setPathID(req *http.Request, id string) *http.Request {
	req.SetPathValue("id", id)
	return req
}
