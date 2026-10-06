// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/credential_test.go
// @for       The assembly of the credential one upstream call presents, from the stored account.
// @uses      internal/domain, internal/provider, testing, time.
// @reason    SPEC-API-001 §6 keeps every credential as ciphertext until the moment it is sent, and §8.1 makes the endpoint's own auth type decide the shape. The non-secret identity beside it is what a signed provider reads on every call, a machine id a device login minted and nothing else carries, so the assembly is worth pinning: a field dropped here turns into a rejected request upstream, far from its cause.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package dataplane

import (
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

var credentialTestNow = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

// buildOAuthEndpoint assembles an account whose credential came from a flow, with
// the identity a device login persisted.
func buildOAuthEndpoint(t *testing.T, account domain.EndpointAccount) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_oauth", "provider-a", "account", domain.UpstreamAuthOAuth, 1, credentialTestNow)
	if err != nil {
		t.Fatalf("building the oauth endpoint: %v", err)
	}
	endpoint.SetOAuth(domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{AccessTokenEncrypted: "at-cipher"}), credentialTestNow)
	endpoint.SetAccount(account, credentialTestNow)
	return endpoint
}

// TestCredentialCarriesTheAccountsMachineID pins the field a signed provider
// replays on every request: the id the login minted reaches the connector through
// the Credential's declared provider slot, and an account without one sends no
// map at all rather than an empty one.
func TestCredentialCarriesTheAccountsMachineID(t *testing.T) {
	endpoint := buildOAuthEndpoint(t, domain.RehydrateEndpointAccount(domain.EndpointAccountInput{
		Email: "dev@example.com", WorkspaceID: "user-7", MachineID: "machine-fixed",
	}))

	credential, err := CredentialForEndpoint(endpoint, domain.UpstreamKey{}, opener{})
	if err != nil {
		t.Fatalf("CredentialForEndpoint() error = %v", err)
	}
	if got := credential.MetadataValue(provider.MetadataMachineID); got != "machine-fixed" {
		t.Fatalf("metadata machine id = %q, want the stored id", got)
	}
	family, secret, err := credential.Secret()
	if err != nil {
		t.Fatalf("Secret() error = %v, want the oauth account resolved cleanly", err)
	}
	if family != provider.FamilyOAuth {
		t.Fatalf("family = %v, want the oauth account to present its token as oauth", family)
	}
	if secret != "plain-at-cipher" {
		t.Fatalf("access token = %q, want the opened value", secret)
	}
	if credential.Account() != "dev@example.com" || credential.ProjectID() != "user-7" {
		t.Fatalf("identity = %q / %q, want the account email and its workspace id",
			credential.Account(), credential.ProjectID())
	}
	if !credential.HasCredential() {
		t.Fatal("the stored oauth token did not reach the credential")
	}
}

// TestCredentialOmitsTheMachineIDWhenTheAccountHasNone pins the other half: a
// key-auth account imported without a machine id reads as no machine id, so a
// connector cannot mistake an empty string for a persisted identity.
func TestCredentialOmitsTheMachineIDWhenTheAccountHasNone(t *testing.T) {
	endpoint := buildEndpoint(t, "ep_key", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "key_1"}})
	key, ok := endpoint.NextKey(credentialTestNow)
	if !ok {
		t.Fatal("the fixture endpoint has no usable key")
	}

	credential, err := CredentialForEndpoint(endpoint, key, opener{})
	if err != nil {
		t.Fatalf("CredentialForEndpoint() error = %v", err)
	}
	if got := credential.MetadataValue(provider.MetadataMachineID); got != "" {
		t.Fatalf("metadata machine id = %q, want none when the account carries no machine id", got)
	}
	family, secret, err := credential.Secret()
	if err != nil {
		t.Fatalf("Secret() error = %v", err)
	}
	if family != provider.FamilyStaticKey {
		t.Fatalf("family = %v, want a key-auth account to present a static key", family)
	}
	if secret != "plain-v1:nonce:cipher" {
		t.Fatalf("api key = %q, want the opened value", secret)
	}
}

// TestCredentialForNoAuthAccountSendsNothing pins the family the gateway serves
// without a credential: no secret, no identity, and no metadata.
func TestCredentialForNoAuthAccountSendsNothing(t *testing.T) {
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_free", "provider-a", "free", domain.UpstreamAuthNone, 1, credentialTestNow)
	if err != nil {
		t.Fatalf("building the no-auth endpoint: %v", err)
	}

	credential, err := CredentialForEndpoint(endpoint, domain.UpstreamKey{}, opener{})
	if err != nil {
		t.Fatalf("CredentialForEndpoint() error = %v", err)
	}
	if _, secret, err := credential.Secret(); err != nil || secret != "" {
		t.Fatalf("a no-auth account presented credential material: %q (%v)", secret, err)
	}
	if got := credential.MetadataValue(provider.MetadataMachineID); got != "" {
		t.Fatalf("a no-auth account presented a machine id %q", got)
	}
}

// credentialSecret is what a test asks a presented credential for: the one value
// the account presents. An account that could not decide is a failed assertion,
// not a value to compare.
func credentialSecret(t *testing.T, cred provider.Credential) string {
	t.Helper()
	_, value, err := cred.Secret()
	if err != nil {
		t.Fatalf("Credential.Secret() error = %v, want one credential family", err)
	}
	return value
}

// credentialFamily names which credential family the account presented, so a
// test can pin the placement rule rather than the field it came from.
func credentialFamily(t *testing.T, cred provider.Credential) provider.Family {
	t.Helper()
	family, _, err := cred.Secret()
	if err != nil {
		t.Fatalf("Credential.Secret() error = %v, want one credential family", err)
	}
	return family
}
