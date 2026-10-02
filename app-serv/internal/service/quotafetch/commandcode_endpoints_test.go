// Command Code wire tests (AGENTS.md §2.1): the three-surface walk and its order, the organization that
// scopes two of those reads, the billing host the entry declares, and the refusal each surface gives.
//
// @file      internal/service/quotafetch/commandcode_endpoints_test.go
// @for       Locks the Command Code requests and their soft answers: three GETs in order on the billing host, one bearer key throughout, and the sentence each refusal becomes.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    The plan ceiling comes from the subscription and the wallet from the billing call, so a walk that stops early, skips a surface or reaches another host shows the operator a balance with nothing beside it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestCommandCode_AsksAllThreeSurfacesWithTheBearerKeyOnce is the regression that catches this family
// calling the wrong host: whoami first, then the two billing reads its organization scopes, one bearer key
// on every one of them, and no surface asked twice.
func TestCommandCode_AsksAllThreeSurfacesWithTheBearerKeyOnce(t *testing.T) {
	stub := commandCodeStubFor(t, commandCodeAnswers(`{"org":{"id":"org-1"}}`,
		`{"credits":{"monthlyCredits":4}}`, `{"data":{"planId":"individual-go"}}`))
	result := stub.read(t, Credentials{APIKey: " user_key_1 ",
		UsageHeaders: map[string]string{"x-command-code-version": "0.25.7"}},
		"GET "+commandCodeWhoamiAPI+"?limits=1; GET "+commandCodeCreditsAPI+"?orgId=org-1; GET "+
			commandCodeSubscriptionsAPI+"?orgId=org-1")

	if len(result.Quotas) != 1 || result.Quotas[0].Total != 10 || result.Quotas[0].Used != 6 {
		t.Fatalf("rows = %+v, want the capped Go window of 10 with 6 spent (%s)", result.Quotas, result.Message)
	}
	for key, want := range map[string]string{"Authorization": "Bearer user_key_1",
		"Accept": "application/json", "x-command-code-version": "0.25.7"} {
		if got := stub.header.Get(key); got != want {
			t.Errorf("header %s = %q, want %q", key, got, want)
		}
	}
	// The seam rewrites the host, so the built-in billing host is pinned here rather than from a request:
	// the commandcode entry declares no usage block, and this is the only host a keyless read can reach.
	if commandCodeAPIBase != "https://api.commandcode.ai" {
		t.Errorf("built-in billing base = %q, want the host the reference defaults to", commandCodeAPIBase)
	}
}

// TestCommandCode_ScopesBothBillingReadsToTheOrganizationWhoamiNames pins the query the two billing routes
// carry: the id as published and escaped, and dropped entirely when the account states none.
func TestCommandCode_ScopesBothBillingReadsToTheOrganizationWhoamiNames(t *testing.T) {
	cases := []struct {
		name    string
		whoami  string
		wantArg string
	}{
		{name: "an organization with a path separator is escaped", whoami: `{"org":{"id":"org/42"}}`,
			wantArg: "?orgId=org%2F42"},
		{name: "a numeric organization is asked as published", whoami: `{"org":{"id":42}}`, wantArg: "?orgId=42"},
		{name: "a null organization is no parameter at all", whoami: `{"org":{"id":null}}`},
		{name: "an empty organization is no parameter", whoami: `{"org":{"id":""}}`},
		{name: "an account that states no organization asks without one", whoami: `{}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := commandCodeStubFor(t, commandCodeAnswers(testCase.whoami, `{}`, `{}`))
			stub.read(t, Credentials{APIKey: "k"}, "GET "+commandCodeWhoamiAPI+"?limits=1; GET "+
				commandCodeCreditsAPI+testCase.wantArg+"; GET "+commandCodeSubscriptionsAPI+testCase.wantArg)
		})
	}
}

// TestCommandCode_ReadsItsHostFromTheDeclaredURLOrTheBuiltIn pins both halves of the fallback: a declared
// usage url is the billing base and keeps each route's own path, and the built-in is the answer only when
// the entry declares nothing.
func TestCommandCode_ReadsItsHostFromTheDeclaredURLOrTheBuiltIn(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		wantBase string
	}{
		{name: "the built-in base when nothing is declared"},
		{name: "the declared usage url is the base, without its trailing slash", declared: "/moved/billing/",
			wantBase: "/moved/billing"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := commandCodeStubFor(t, commandCodeAnswers(`{}`, `{}`, `{}`))
			creds := Credentials{APIKey: "k"}
			if testCase.declared != "" {
				creds.Endpoints.URL = stub.server.URL + testCase.declared
			} else {
				creds.Endpoint = stub.server.URL
			}
			fetchCommandCode(context.Background(), creds)

			want := "GET " + testCase.wantBase + commandCodeWhoamiAPI + "?limits=1; GET " +
				testCase.wantBase + commandCodeCreditsAPI + "; GET " + testCase.wantBase + commandCodeSubscriptionsAPI
			if got := stub.requests(); got != want {
				t.Fatalf("requests = %q, want %q", got, want)
			}
		})
	}
}

// TestCommandCode_SoftAnswers pins the family's refusals: nothing asked without a key, a dead credential
// named once where it was refused, and each surface that errored named by its own route.
func TestCommandCode_SoftAnswers(t *testing.T) {
	walkWhoami := "GET " + commandCodeWhoamiAPI + "?limits=1"
	walkCredits := walkWhoami + "; GET " + commandCodeCreditsAPI + "?orgId=o-1"
	walkAll := walkCredits + "; GET " + commandCodeSubscriptionsAPI + "?orgId=o-1"
	cases := []struct {
		name      string
		creds     Credentials
		byRoute   map[string]commandCodeFixture
		wantAsked string
		wantPlan  string
		want      string
		starts    string
	}{
		{name: "no key asks the provider nothing", byRoute: commandCodeAnswers(`{}`, `{}`, `{}`),
			want: "Command Code API key not available. Add a key to view usage."},
		{name: "a key of only spaces asks nothing either", creds: Credentials{APIKey: "  "},
			byRoute: commandCodeAnswers(`{}`, `{}`, `{}`),
			want:    "Command Code API key not available. Add a key to view usage."},
		{name: "a 401 on whoami stops the walk at the credential", creds: Credentials{APIKey: "k"},
			byRoute:   commandCodeRefusing(commandCodeWhoamiAPI, http.StatusUnauthorized),
			wantAsked: walkWhoami, wantPlan: commandCodeDisplay,
			want: "Command Code authentication failed. Check the API key."},
		{name: "a 403 on the credits read stops it there", creds: Credentials{APIKey: "k"},
			byRoute: commandCodeRefusing(commandCodeCreditsAPI, http.StatusForbidden), wantAsked: walkCredits,
			wantPlan: commandCodeDisplay, want: "Command Code authentication failed. Check the API key."},
		{name: "a 403 on the subscription read is the same dead key", creds: Credentials{APIKey: "k"},
			byRoute: commandCodeRefusing(commandCodeSubscriptionsAPI, http.StatusForbidden), wantAsked: walkAll,
			wantPlan: commandCodeDisplay, want: "Command Code authentication failed. Check the API key."},
		{name: "a 500 on whoami names the usage surface", creds: Credentials{APIKey: "k"},
			byRoute:   commandCodeRefusing(commandCodeWhoamiAPI, http.StatusInternalServerError),
			wantAsked: walkWhoami, wantPlan: commandCodeDisplay, want: "Command Code usage API error (500)"},
		{name: "a 502 on credits names the credits surface", creds: Credentials{APIKey: "k"},
			byRoute: commandCodeRefusing(commandCodeCreditsAPI, http.StatusBadGateway), wantAsked: walkCredits,
			wantPlan: commandCodeDisplay, want: "Command Code credits API error (502)"},
		{name: "a 503 on subscriptions names the subscription surface", creds: Credentials{APIKey: "k"},
			byRoute:   commandCodeRefusing(commandCodeSubscriptionsAPI, http.StatusServiceUnavailable),
			wantAsked: walkAll, wantPlan: commandCodeDisplay,
			want: "Command Code subscriptions API error (503)"},
		{name: "an unreachable provider is a sentence, not a panic", wantPlan: commandCodeDisplay,
			creds:  Credentials{APIKey: "k", Endpoints: UsageEndpoints{URL: "https://api.commandcode.ai\x7f"}},
			starts: "Command Code error: "},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := commandCodeStubFor(t, testCase.byRoute)
			creds := testCase.creds
			if creds.Endpoints.URL == "" {
				creds.Endpoint = stub.server.URL
			}
			result := fetchCommandCode(context.Background(), creds)

			if got := stub.requests(); got != testCase.wantAsked {
				t.Fatalf("requests = %q, want %q", got, testCase.wantAsked)
			}
			if len(result.Quotas) != 0 {
				t.Fatalf("rows = %+v, want a soft answer", result.Quotas)
			}
			if result.Plan != testCase.wantPlan {
				t.Errorf("plan = %q, want %q", result.Plan, testCase.wantPlan)
			}
			if testCase.want != "" && result.Message != testCase.want {
				t.Fatalf("message = %q, want %q", result.Message, testCase.want)
			}
			if testCase.starts != "" && !strings.HasPrefix(result.Message, testCase.starts) {
				t.Fatalf("message = %q, want it to start with %q", result.Message, testCase.starts)
			}
		})
	}
}

// commandCodeRefusing answers one route with a refusal and the other two with an empty body, so the walk a
// soft answer reports is pinned to the surface that actually refused.
func commandCodeRefusing(route string, status int) map[string]commandCodeFixture {
	byRoute := commandCodeAnswers(`{"org":{"id":"o-1"}}`, `{}`, `{}`)
	byRoute[route] = commandCodeFixture{status: status, body: `{"message":"refused"}`}
	return byRoute
}
