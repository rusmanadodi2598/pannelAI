// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_account_test.go
// @for       The account identity's canonical spelling and the credential constructor.
// @uses      testing, time.
// @reason    Two rules decide whether a re-import is recognised as the account it already is: the email's canonical form and a credential that cannot exist without the ciphertext it names.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-04
package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"Bob@Example.COM":             "bob@example.com",
		"  bob@example.com  ":         "bob@example.com",
		"":                            "",
		"\tMixed.Case@Host.IO\n":      "mixed.case@host.io",
		"UPPER@SUB.DOMAIN.COM":        "upper@sub.domain.com",
		"trailing space kept inside ": "trailing space kept inside",
	}
	for raw, want := range cases {
		if got := NormalizeEmail(raw); got != want {
			t.Fatalf("NormalizeEmail(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestUpstreamEndpointSetAccountStoresTheCanonicalEmail(t *testing.T) {
	now := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	endpoint, err := NewUpstreamEndpoint("ep_1", "qoder", "acct", UpstreamAuthOAuth, 1, now)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	account, err := NewEndpointAccount(EndpointAccountInput{
		Name: "Qoder", Email: " Bob@Example.COM ", WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("NewEndpointAccount() error = %v", err)
	}
	endpoint.SetAccount(account, now)

	if got := endpoint.Account().Email().String(); got != "bob@example.com" {
		t.Fatalf("Account().Email = %q, want the canonical spelling the duplicate check matches on", got)
	}
}

func TestNewOAuthCredential(t *testing.T) {
	now := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)

	if _, err := NewOAuthCredential(OAuthCredentialInput{AccountEmail: "a@b.c"}); err == nil {
		t.Fatal("NewOAuthCredential() = nil error for a credential with no access ciphertext")
	} else if code := AsAppError(err).Code; code != "VALIDATION_ERROR" {
		t.Fatalf("NewOAuthCredential() code = %q, want VALIDATION_ERROR", code)
	}

	credential, err := NewOAuthCredential(OAuthCredentialInput{
		AccessTokenEncrypted: "enc-access", RefreshTokenEncrypted: " enc-refresh ",
		AccountEmail: "Ana@Example.COM", LastRefreshAt: &now,
	})
	if err != nil {
		t.Fatalf("NewOAuthCredential() error = %v", err)
	}
	if credential.RefreshTokenEncrypted() != "enc-refresh" || credential.AccountEmail().String() != "ana@example.com" {
		t.Fatalf("credential = %v, want the trimmed token and the canonical email", credential)
	}
}

func TestParseEmail(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		err  bool
	}{
		{raw: "Bob@Example.COM", want: "bob@example.com"},
		{raw: "  a@b.c  ", want: "a@b.c"},
		{raw: "qoder-user-12345", want: "qoder-user-12345"},
		{raw: "", want: ""},
		{raw: "two words@example.com", err: true},
		{raw: strings.Repeat("a", maxEmailLength+1) + "@example.com", err: true},
	}
	for _, tc := range cases {
		parsed, err := ParseEmail(tc.raw)
		if tc.err {
			if err == nil {
				t.Fatalf("ParseEmail(%q) = %q with no error, want a refusal", truncateForTest(tc.raw), parsed)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseEmail(%q) error = %v, want none", truncateForTest(tc.raw), err)
		}
		if got := parsed.String(); got != tc.want {
			t.Fatalf("ParseEmail(%q) = %q, want %q", truncateForTest(tc.raw), got, tc.want)
		}
	}
}

func TestEndpointAccountRejectsAnUnusableEmail(t *testing.T) {
	if _, err := NewEndpointAccount(EndpointAccountInput{Name: "box", Email: "two words"}); err == nil {
		t.Fatal("NewEndpointAccount() accepted an identity containing a space")
	}
}

// truncateForTest keeps a failure message readable when a case carries a
// deliberately oversized value.
func truncateForTest(raw string) string {
	if len(raw) > 40 {
		return raw[:40] + "..."
	}
	return raw
}
