// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_google_accounts_test.go
// @for       Several real vendor accounts of one provider, each keeping its own tokens and its own Cloud project.
// @uses      context, fmt, testing, time, internal/domain, internal/service/oauthhttp.
// @reason    The identity-less multi-account rule was already pinned, but a Google login is the other case: the vendor states an email, so a second account must create a second row while a re-login of the first must update it, and each row carries a project that belongs to its own account. Written as one test because the failure this guards is the one draft 011 already caught once: a dedup key that lets login two overwrite account one.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-18
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
)

// sequencedProjects hands each login the project its own bootstrap answered with,
// so a test can tell whose project ended up on a row.
type sequencedProjects struct {
	calls []string
	next  int
}

func (s *sequencedProjects) ResolveProject(_ context.Context, providerID, _ string) (string, bool) {
	s.calls = append(s.calls, providerID)
	s.next++
	return "proj-" + string(rune('0'+s.next)), true
}

// connectGoogleAccount runs one full code round as the vendor would answer it: the
// token endpoint issues a distinct token, and userinfo names the account by email.
func connectGoogleAccount(t *testing.T, fixture oauthFlowFixture, email string) OAuthConnect {
	t.Helper()
	fixture.tokens.infoFn = func() (oauthhttp.OAuthIdentity, error) {
		return oauthhttp.OAuthIdentity{Email: email, Name: email}, nil
	}
	issued := "at-" + email
	fixture.tokens.grantFn = func(oauthhttp.TokenGrant) (oauthhttp.TokenResponse, error) {
		return oauthhttp.TokenResponse{
			AccessToken: issued, RefreshToken: "rt-" + email, ExpiresIn: 3600,
		}, nil
	}
	connected, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "gc", Code: "code-" + email, State: startFlow(t, fixture, "gc"),
	})
	if err != nil {
		t.Fatalf("Callback() for %s: %v", email, err)
	}
	return connected
}

func TestOAuthCallback_TwoVendorAccountsGetTwoRowsEachWithItsOwnProject(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithBootstrap("gc"))
	resolver := &sequencedProjects{}
	fixture.service.projects = resolver

	first := connectGoogleAccount(t, fixture, "one@example.com")
	second := connectGoogleAccount(t, fixture, "two@example.com")

	if !first.Created || !second.Created {
		t.Fatalf("created = %v / %v, want both logins to land their own row", first.Created, second.Created)
	}
	if first.Endpoint.ID() == second.Endpoint.ID() {
		t.Fatalf("both accounts wrote the same endpoint %s: the second login overwrote the first", first.Endpoint.ID())
	}
	// The project a bootstrap resolved for one account must not reach the other.
	if got := first.Endpoint.OAuth().ProjectID(); got != "proj-1" {
		t.Fatalf("account one project = %q, want proj-1", got)
	}
	if got := second.Endpoint.OAuth().ProjectID(); got != "proj-2" {
		t.Fatalf("account two project = %q, want proj-2", got)
	}
	if len(resolver.calls) != 2 {
		t.Fatalf("bootstrap calls = %v, want one per login", resolver.calls)
	}

	// The two rows really are distinct secrets, not one credential read twice.
	openedOne, err := fixture.sealer.Open(first.Endpoint.OAuth().AccessTokenEncrypted())
	if err != nil {
		t.Fatalf("opening account one's token: %v", err)
	}
	openedTwo, err := fixture.sealer.Open(second.Endpoint.OAuth().AccessTokenEncrypted())
	if err != nil {
		t.Fatalf("opening account two's token: %v", err)
	}
	if openedOne != "at-one@example.com" || openedTwo != "at-two@example.com" {
		t.Fatalf("stored access tokens = %q / %q, want each account's own", openedOne, openedTwo)
	}
}

// TestOAuthCallback_ReLoginOfTheSameEmailUpdatesItsRow holds the other half of the
// rule: the email is the dedup key, so signing into a known account renews that row
// rather than adding a second one the router would then rotate between.
func TestOAuthCallback_ReLoginOfTheSameEmailUpdatesItsRow(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithBootstrap("gc"))
	fixture.service.projects = &sequencedProjects{}

	original := connectGoogleAccount(t, fixture, "one@example.com")
	again := connectGoogleAccount(t, fixture, "one@example.com")

	if again.Created {
		t.Fatal("re-login created a second row for an account the store already had")
	}
	if again.Endpoint.ID() != original.Endpoint.ID() {
		t.Fatalf("re-login wrote %s, want the existing account %s", again.Endpoint.ID(), original.Endpoint.ID())
	}
	if again.Endpoint.OAuth().ProjectID() != "proj-2" {
		t.Fatalf("project after re-login = %q, want the freshly resolved one", again.Endpoint.OAuth().ProjectID())
	}

	rows := 0
	for _, endpoint := range fixture.store.byID {
		if endpoint.ProviderID() == "gc" {
			rows++
		}
	}
	if rows != 1 {
		t.Fatalf("provider has %d rows, want one account stored once", rows)
	}
}

// TestOAuthCallback_AccountsAreRefreshedPerRow is why two rows matter at all: the
// sweep renews each account on its own schedule, so one spent refresh token cannot
// strand the other.
func TestOAuthCallback_AccountsAreRefreshedPerRow(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithBootstrap("gc"))
	fixture.service.projects = &sequencedProjects{}
	first := connectGoogleAccount(t, fixture, "one@example.com")
	second := connectGoogleAccount(t, fixture, "two@example.com")

	if _, err := fixture.service.Refresh(context.Background(), "gc", second.Endpoint.ID()); err != nil {
		t.Fatalf("Refresh() on the second account: %v", err)
	}
	if len(fixture.tokens.grantCalls) != 3 {
		t.Fatalf("grant calls = %d, want the two logins plus one renewal", len(fixture.tokens.grantCalls))
	}
	last := fixture.tokens.grantCalls[len(fixture.tokens.grantCalls)-1]
	if last.GrantType != "refresh_token" || last.RefreshToken != "rt-two@example.com" {
		t.Fatalf("renewal grant = %+v, want the second account's own refresh token", last)
	}

	reloaded, err := fixture.store.GetByID(context.Background(), first.Endpoint.ID())
	if err != nil {
		t.Fatalf("reloading the first account: %v", err)
	}
	if held := reloaded.OAuth(); held == nil || held.AccessTokenEncrypted() != first.Endpoint.OAuth().AccessTokenEncrypted() {
		t.Fatal("renewing one account moved the other account's stored credential")
	}
}
