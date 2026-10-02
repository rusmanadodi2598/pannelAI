// Claude legacy tests: the organization fallback the primary path hands over to, the {org_id}
// the settings read supplies, and the two admin sentences the reference answers when the
// fallback has nothing to show.
//
// @file      internal/service/quotafetch/claude_org_test.go
// @for       Locks Claude's settings and organization-usage fallback and its admin-access outcomes.
// @uses      internal/service/quotafetch, context, net/http, strings, testing
// @reason    The fallback changes host twice over and fills a placeholder the registry declares
//
//	inline, so a read that reaches the wrong organization, or that reports an
//	admin-only account as broken, is the failure this path has to keep out.
//
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

// A primary that answers 2xx with nothing readable is the reference's trigger for the legacy
// read, and the org URL's placeholder is filled from the settings response, not from the
// caller.
func TestClaudeFallsBackToOrganizationUsage(t *testing.T) {
	stub := newClaudeStub(t, map[string]claudeReply{
		"/api/oauth/usage":              {body: `{"limits":[]}`},
		"/v1/settings":                  {body: claudeSettingsBody},
		"/v1/organizations/org-7/usage": {body: claudeUsageBody},
	})

	result := fetchClaude(context.Background(), Credentials{AccessToken: "tok", Endpoint: stub.server.URL})

	wantCalls := []string{"GET /api/oauth/usage", "GET /v1/settings", "GET /v1/organizations/org-7/usage"}
	if got := stub.recorded(); strings.Join(got, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("calls = %v, want %v", got, wantCalls)
	}
	// The organization document is read by the same window reader, so the fallback keeps the
	// plan the settings read named rather than the OAuth plan label.
	if result.Plan != "claude_max" || len(result.Quotas) != 5 {
		t.Fatalf("plan=%q rows=%+v (%s)", result.Plan, result.Quotas, result.Message)
	}
}

func TestClaudeReadsADeclaredOrgURLWithoutAPlaceholder(t *testing.T) {
	stub := newClaudeStub(t, map[string]claudeReply{
		"/api/oauth/usage":         {status: http.StatusNotFound},
		"/v1/settings":             {body: claudeSettingsBody},
		"/v1/usage/for/this/entry": {body: claudeUsageBody},
	})
	creds := Credentials{
		AccessToken: "tok",
		Endpoint:    stub.server.URL,
		Endpoints:   UsageEndpoints{OrgURL: stub.server.URL + "/v1/usage/for/this/entry"},
	}

	result := fetchClaude(context.Background(), creds)

	// An operator who declared an exact address gets that address; inventing an organization
	// segment inside it would move the read somewhere else entirely.
	if got := stub.recorded()[len(stub.recorded())-1]; got != "GET /v1/usage/for/this/entry" {
		t.Fatalf("calls = %v, want the declared organization path", got)
	}
	if len(result.Quotas) != 5 {
		t.Fatalf("rows = %+v (%s)", result.Quotas, result.Message)
	}
}

func TestClaudeLegacySentences(t *testing.T) {
	cases := []struct {
		name        string
		replies     map[string]claudeReply
		wantCalls   int
		wantMessage string
	}{
		{
			name: "a settings read without an organization needs admin access",
			replies: map[string]claudeReply{
				"/api/oauth/usage": {status: http.StatusNotFound},
				"/v1/settings":     {body: `{"plan":"pro"}`},
			},
			wantCalls:   2,
			wantMessage: "Claude connected. Usage details require admin access.",
		},
		{
			name: "a settings read that refuses says so",
			replies: map[string]claudeReply{
				"/api/oauth/usage": {status: http.StatusInternalServerError},
				"/v1/settings":     {status: http.StatusForbidden},
			},
			wantCalls:   2,
			wantMessage: "Claude connected. Usage API requires admin permissions.",
		},
		{
			name: "an organization read that refuses keeps the plan it learned",
			replies: map[string]claudeReply{
				"/api/oauth/usage":              {body: `{}`},
				"/v1/settings":                  {body: claudeSettingsBody},
				"/v1/organizations/org-7/usage": {status: http.StatusNotFound},
			},
			wantCalls:   3,
			wantMessage: "Claude connected. Usage details require admin access.",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := newClaudeStub(t, testCase.replies)

			result := fetchClaude(context.Background(), Credentials{AccessToken: "tok", Endpoint: stub.server.URL})

			if got := stub.recorded(); len(got) != testCase.wantCalls {
				t.Fatalf("calls = %v, want %d", got, testCase.wantCalls)
			}
			if result.Message != testCase.wantMessage || len(result.Quotas) != 0 {
				t.Fatalf("message = %q rows = %+v, want %q", result.Message, result.Quotas, testCase.wantMessage)
			}
		})
	}
}
