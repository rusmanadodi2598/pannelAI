// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_create_test.go
// @for       The credential sealing and the duplicate-account refusal on create.
// @uses      testing, internal/domain.
// @reason    A create is the only moment a plaintext credential exists in this service, so the seal and the uniqueness rule are pinned apart from the CRUD table.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestEndpointService_CreateSealsTheCredential asserts the plaintext never reaches
// the aggregate and cannot be recovered from what is stored, the boundary
// SPEC-API-001 §6 draws.
func TestEndpointService_CreateSealsTheCredential(t *testing.T) {
	const secret = "sk-super-secret-value-do-not-store"
	svc, store := newEndpointSvc(t)

	endpoint, err := svc.Create(context.Background(), CreateInput{
		ProviderID: "deepseek", Label: "who", AuthType: domain.UpstreamAuthAPIKey,
		Keys: []KeyInput{{Value: secret}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	key := endpoint.Keys()[0]
	if key.EncryptedValue() == secret {
		t.Fatal("the stored value must never be the plaintext")
	}
	if !isSealed(key.EncryptedValue()) {
		t.Fatalf("stored value = %q, want v1: ciphertext", key.EncryptedValue())
	}
	// The hint reveals the family and the tail only; the body must be absent.
	if key.Hint() == secret {
		t.Fatal("the hint must not be the plaintext")
	}
	stored, ok := store.byID[endpoint.ID()]
	if !ok {
		t.Fatal("the endpoint was not stored")
	}
	if stored.Keys()[0].EncryptedValue() != key.EncryptedValue() {
		t.Fatal("the stored aggregate must carry the sealed value")
	}
}

// TestEndpointService_CreateRejectsDuplicateAccount pins the (provider_id, label)
// rule whose database expression is a unique index, so a caller gets CONFLICT rather
// than a driver message.
func TestEndpointService_CreateRejectsDuplicateAccount(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	in := CreateInput{ProviderID: "deepseek", Label: "primary", AuthType: domain.UpstreamAuthOAuth}

	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	_, err := svc.Create(context.Background(), in)
	mustAppError(t, err, "CONFLICT")
	if !errors.Is(err, domain.ErrEndpointExists) {
		t.Fatalf("error = %v, want it to wrap ErrEndpointExists", err)
	}

	// A different provider may reuse the label: the rule is per provider.
	other, _ := newEndpointSvc(t, "deepseek", "openrouter")
	if _, err := other.Create(context.Background(), CreateInput{
		ProviderID: "openrouter", Label: "primary", AuthType: domain.UpstreamAuthOAuth,
	}); err != nil {
		t.Fatalf("Create() on a second provider error = %v", err)
	}
}
