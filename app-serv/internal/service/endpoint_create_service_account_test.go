// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_create_service_account_test.go
// @for       The create rules of a service_account endpoint: its project and location are required, and they are refused on every other auth type.
// @uses      testing, internal/domain.
// @reason    The service-account fields are the only create inputs whose validity depends on the auth type, so the pairing is pinned here: a stored project with no project, or a project stored on an api_key endpoint, are both states the router would only discover as a wrong URL or a silently ignored field.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestEndpointService_CreateServiceAccountEndpoint pins the happy path: the
// project and location reach the stored aggregate, and the credential is stored
// like any other key.
func TestEndpointService_CreateServiceAccountEndpoint(t *testing.T) {
	svc, store := newEndpointSvc(t, "vertex")

	endpoint, err := svc.Create(context.Background(), CreateInput{
		ProviderID: "vertex", Label: "veo", AuthType: domain.UpstreamAuthServiceAccount,
		SAProjectID: "pannelai-prod", SALocation: "us-central1",
		Keys: []KeyInput{{Value: `{"type":"service_account","project_id":"pannelai-prod"}`}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if endpoint.AuthType() != domain.UpstreamAuthServiceAccount {
		t.Fatalf("auth_type = %q, want service_account", endpoint.AuthType())
	}
	got := endpoint.ServiceAccount()
	if got.ProjectID != "pannelai-prod" || got.Location != "us-central1" {
		t.Fatalf("ServiceAccount() = %+v, want the pair the request stated", got)
	}
	if len(endpoint.Keys()) != 1 {
		t.Fatalf("keys = %d, want the one service-account JSON", len(endpoint.Keys()))
	}

	// The store round-trip carries both fields: a service that returned them but
	// stored nothing would read back unset on the next load.
	stored, ok := store.byID[endpoint.ID()]
	if !ok {
		t.Fatal("the endpoint was not stored")
	}
	if sa := stored.ServiceAccount(); sa.ProjectID != "pannelai-prod" || sa.Location != "us-central1" {
		t.Fatalf("stored ServiceAccount() = %+v, want the pair the request stated", sa)
	}
}

// TestEndpointService_CreateRefusesServiceAccountMismatches pins the three
// refusals: a service_account endpoint without its project, without a key, and
// the fields on an auth type that never reads them.
func TestEndpointService_CreateRefusesServiceAccountMismatches(t *testing.T) {
	cases := []struct {
		name string
		in   CreateInput
	}{
		{
			name: "a service_account endpoint without a project",
			in: CreateInput{ProviderID: "vertex", Label: "no-project", AuthType: domain.UpstreamAuthServiceAccount,
				Keys: []KeyInput{{Value: "{}"}}},
		},
		{
			name: "a service_account endpoint without a location",
			in: CreateInput{ProviderID: "vertex", Label: "no-location", AuthType: domain.UpstreamAuthServiceAccount,
				SAProjectID: "pannelai-prod", Keys: []KeyInput{{Value: "{}"}}},
		},
		{
			name: "a service_account endpoint without a key",
			in: CreateInput{ProviderID: "vertex", Label: "keyless", AuthType: domain.UpstreamAuthServiceAccount,
				SAProjectID: "pannelai-prod", SALocation: "us-central1"},
		},
		{
			name: "the fields on an api_key endpoint",
			in: CreateInput{ProviderID: "deepseek", Label: "wrong-type", AuthType: domain.UpstreamAuthAPIKey,
				SAProjectID: "pannelai-prod", SALocation: "us-central1",
				Keys: []KeyInput{{Value: "sk-a"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newEndpointSvc(t, "vertex")
			_, err := svc.Create(context.Background(), tc.in)
			mustAppError(t, err, "VALIDATION_ERROR")
			if len(store.byID) != 0 {
				t.Fatalf("stored endpoints = %d, want none: a refused create must write nothing", len(store.byID))
			}
		})
	}
}
