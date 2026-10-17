// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/credential_project_test.go
// @for       Which slot a stored Cloud project reaches the connector through.
// @uses      domain, internal/provider, testing.
// @reason    The Credential had one account-identity field and two writers: the workspace id a signed provider reads as its user id, and a Google login's Cloud project, which overwrote it whenever the account happened to have both. The project is neither a secret nor a workspace, so it now rides the metadata slot the machine id already uses, and the field a Qoder request depends on stops depending on which provider was connected last.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-18
package dataplane

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

func TestCredentialPutsTheProjectInItsOwnSlot(t *testing.T) {
	endpoint := buildOAuthEndpoint(t, domain.RehydrateEndpointAccount(domain.EndpointAccountInput{
		Email: "dev@example.com", WorkspaceID: "user-7",
	}))
	endpoint.SetOAuth(domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{
		AccessTokenEncrypted: "at-cipher", ProjectID: "proj-9",
	}), credentialTestNow)

	credential, err := CredentialForEndpoint(endpoint, domain.UpstreamKey{}, opener{})
	if err != nil {
		t.Fatalf("CredentialForEndpoint() error = %v", err)
	}
	if got := credential.MetadataValue(provider.MetadataProjectID); got != "proj-9" {
		t.Fatalf("metadata project = %q, want the stored project under the provider's declared key", got)
	}
	// The slot a signed provider reads as its user id must keep carrying the
	// workspace, whatever else the account stores.
	if got := credential.ProjectID(); got != "user-7" {
		t.Fatalf("ProjectID() = %q, want the account workspace and not the Google project", got)
	}
}

func TestCredentialSendsNoProjectKeyForAnAccountWithoutOne(t *testing.T) {
	endpoint := buildOAuthEndpoint(t, domain.RehydrateEndpointAccount(domain.EndpointAccountInput{
		Email: "dev@example.com",
	}))

	credential, err := CredentialForEndpoint(endpoint, domain.UpstreamKey{}, opener{})
	if err != nil {
		t.Fatalf("CredentialForEndpoint() error = %v", err)
	}
	if got := credential.MetadataValue(provider.MetadataProjectID); got != "" {
		t.Fatalf("metadata project = %q, want nothing sent for an account that stores none", got)
	}
}

// TestCredentialKeepsAKeyAccountsProjectless pins the branch that matters for a
// Google API-key endpoint: its project, if any, is an OAuth credential fact, so a
// static key must not gain a project slot it never had.
func TestCredentialKeepsAKeyAccountsProjectless(t *testing.T) {
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_key", "gemini", "account", domain.UpstreamAuthAPIKey, 1, credentialTestNow)
	if err != nil {
		t.Fatalf("building the key endpoint: %v", err)
	}
	key := domain.RehydrateUpstreamKey(
		"uky_key", "ep_key", "key", "key-cipher", "…hint", 1,
		domain.UpstreamKeyActive, nil, "", 0, domain.KeyRotationState{}, nil,
		credentialTestNow, credentialTestNow, domain.OperatorState{})

	credential, err := CredentialForEndpoint(endpoint, key, opener{})
	if err != nil {
		t.Fatalf("CredentialForEndpoint() error = %v", err)
	}
	if got := credential.MetadataValue(provider.MetadataProjectID); got != "" {
		t.Fatalf("metadata project = %q, want none for a static key", got)
	}
	if family, _, err := credential.Secret(); err != nil || family != provider.FamilyStaticKey {
		t.Fatalf("family = %v (%v), want the key to present as a static credential", family, err)
	}
}
