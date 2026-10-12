// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_response_service_account_test.go
// @for       That an endpoint's service-account fields reach the §7.5 wire shape the panel renders, and stay absent on every other auth type.
// @uses      testing, internal/domain.
// @reason    The project and location are the only non-secret fields a service_account endpoint carries, and a mapper that dropped them would leave the panel with nothing to show for an account it just created. The negative case is pinned beside it because a rendered empty project on an api_key endpoint would read as stored configuration.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-12
package handler

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestToEndpointResponse_CarriesTheServiceAccountFields pins the §7.5 mapping
// for a service_account endpoint.
func TestToEndpointResponse_CarriesTheServiceAccountFields(t *testing.T) {
	endpoint, err := domain.NewUpstreamEndpoint("ep_a", "vertex", "veo",
		domain.UpstreamAuthServiceAccount, 1, now)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}
	if err := endpoint.SetServiceAccount(domain.ServiceAccountRef{
		ProjectID: "pannelai-prod", Location: "us-central1",
	}, now); err != nil {
		t.Fatalf("setting the service account: %v", err)
	}

	resp := toEndpointResponse(endpoint, now, false)
	if resp.AuthType != "service_account" {
		t.Fatalf("auth_type = %q, want service_account", resp.AuthType)
	}
	if resp.SAProjectID != "pannelai-prod" {
		t.Fatalf("sa_project_id = %q, want pannelai-prod", resp.SAProjectID)
	}
	if resp.SALocation != "us-central1" {
		t.Fatalf("sa_location = %q, want us-central1", resp.SALocation)
	}
}

// TestToEndpointResponse_ServiceAccountFieldsStayEmpty pins the negative: an
// api_key endpoint renders no service-account members at all.
func TestToEndpointResponse_ServiceAccountFieldsStayEmpty(t *testing.T) {
	endpoint, err := domain.NewUpstreamEndpoint("ep_a", "deepseek", "primary",
		domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}

	resp := toEndpointResponse(endpoint, now, false)
	if resp.SAProjectID != "" || resp.SALocation != "" {
		t.Fatalf("service-account fields = %q/%q, want both empty", resp.SAProjectID, resp.SALocation)
	}
}
