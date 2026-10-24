// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_oauth_jsonb_test.go
// @for       The stored shape of an endpoint's OAuth credential set, proven by writing it and reading it back.
// @uses      domain, strings, testing.
// @reason    The oauth column is JSONB decoded through a payload struct, so a field that struct does not name disappears without an error: the login succeeds, the read returns nothing for that field, and the loss surfaces later as a call scoped to no project. The project is exactly such a field, and its absence must stay absent rather than become an empty value a provider can read as a real blank.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-17
package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func oauthFixture(project string) *domain.OAuthCredential {
	expires := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	return domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{
		AccessTokenEncrypted:  "sealed-access",
		RefreshTokenEncrypted: "sealed-refresh",
		ExpiresAt:             &expires,
		Scopes:                []string{"scope:one"},
		ProjectID:             project,
		AccountEmail:          "dev@example.com",
	})
}

func TestOAuthRoundTripKeepsTheProject(t *testing.T) {
	raw, err := marshalOAuth(oauthFixture("proj-9"))
	if err != nil {
		t.Fatalf("marshalOAuth() error = %v", err)
	}
	if raw == nil || !strings.Contains(*raw, `"project_id":"proj-9"`) {
		t.Fatalf("stored oauth = %v, want the project under the key the lookup matches on", raw)
	}

	read, err := unmarshalOAuth(raw)
	if err != nil {
		t.Fatalf("unmarshalOAuth() error = %v", err)
	}
	if read.ProjectID() != "proj-9" {
		t.Fatalf("project after the round trip = %q, want proj-9", read.ProjectID())
	}
	if read.AccessTokenEncrypted() != "sealed-access" || read.RefreshTokenEncrypted() != "sealed-refresh" {
		t.Fatal("the token set did not survive alongside the project")
	}
}

// TestOAuthProjectIsAbsentRatherThanBlank keeps the two states distinguishable
// on disk: a Google account whose bootstrap has not answered yet must not store
// a project a provider could read as a real, empty one.
func TestOAuthProjectIsAbsentRatherThanBlank(t *testing.T) {
	raw, err := marshalOAuth(oauthFixture(""))
	if err != nil {
		t.Fatalf("marshalOAuth() error = %v", err)
	}
	if strings.Contains(*raw, "project_id") {
		t.Fatalf("stored oauth %q carries a project key for an account that has none", *raw)
	}
	read, err := unmarshalOAuth(raw)
	if err != nil {
		t.Fatalf("unmarshalOAuth() error = %v", err)
	}
	if read.ProjectID() != "" {
		t.Fatalf("project after the round trip = %q, want empty", read.ProjectID())
	}
}
