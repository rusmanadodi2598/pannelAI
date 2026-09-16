// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/gateway_key_revoke_test.go
// @for       HTTP tests for DELETE /api/v1/gateway-keys/{id}.
// @uses      internal/handler, internal/schema, internal/service, internal/domain,
//            internal/repository, net/http/httptest.
// @reason    SPEC-API-001 §7.3 makes revocation terminal, so the test asserts the revoke instant is recorded and a second revoke is refused rather than silently succeeding.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-16

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestGatewayKey_Revoke_IsTerminal covers DELETE then a second revoke.
func TestGatewayKey_Revoke_IsTerminal(t *testing.T) {
	h, repo := newTestHandler(t)
	key := domain.NewGatewayKey("ci", "sk-secret-value", "sk-…alue", time.Now())
	if err := repo.Create(context.Background(), key); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := setPathID(httptest.NewRequest(http.MethodDelete, "/api/v1/gateway-keys/"+key.ID(), nil), key.ID())
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rr.Code)
	}

	stored, err := repo.GetByID(context.Background(), key.ID())
	if err != nil {
		t.Fatalf("load after revoke: %v", err)
	}
	if stored.Status() != domain.GatewayKeyRevoked {
		t.Fatalf("status = %q, want revoked", stored.Status())
	}
	if stored.RevokedAt() == nil {
		t.Fatal("revoked_at must be set")
	}

	rr2 := httptest.NewRecorder()
	h.Delete(rr2, req)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("second delete status = %d, want 409", rr2.Code)
	}
}
