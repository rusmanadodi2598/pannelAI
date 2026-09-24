// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/systemone_boundary_test.go
// @for       The decision route's boundary rules: the bodies it refuses before
//
//	calling, the combos it refuses, and the URLs it will not dial.
//
// @uses      context, strings, testing, time, internal/dataplane,
//
//	internal/domain, internal/provider, internal/registry, internal/schema.
//
// @reason    These cases are about what never reaches an upstream, which is a
//
//	different question from what the route forwards. Keeping them apart
//	also holds systemone_test.go inside the AGENTS.md §1.1 budget, and it
//	puts the reference's own boundary rules (systemoneCore.js:30-36) in
//	one readable list.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestSystemOne_PlacesTheAccountCredentialAsABearer pins the keyed lane: an
// account holding a key presents it as a bearer, which is what the reference
// sends when the connection has one (systemoneCore.js:39-44).
func TestSystemOne_PlacesTheAccountCredentialAsABearer(t *testing.T) {
	cases := []struct {
		name string
		cred provider.Credential
		want string
	}{
		{name: "a static key", cred: provider.StaticKey("ep", "k1", "sk-live"), want: "Bearer sk-live"},
		{name: "an oauth token", cred: provider.OAuthToken("ep", "k1", "ya29.token"), want: "Bearer ya29.token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller := &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"answers":{}}`)}}
			svc, _, _ := systemOneFixture(t, systemOneProvider(), caller, &stubMediaRouter{credential: tc.cred})
			if _, err := svc.Decide(context.Background(), decisionRequest(t), "key-1"); err != nil {
				t.Fatalf("Decide() error = %v", err)
			}
			if got := caller.requests[0].Headers["Authorization"]; got != tc.want {
				t.Fatalf("Authorization = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSystemOne_RefusesAnUnusableBodyBeforeCalling pins the boundary rules the
// reference enforces (systemoneCore.js:30-36), so a body the upstream would
// refuse is rejected here instead of spending the call.
func TestSystemOne_RefusesAnUnusableBodyBeforeCalling(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "no state", raw: `{"model":"opencode/jev-1.13-free","questions":{"q":{"type":"noul"}}}`},
		{name: "a null state", raw: `{"model":"opencode/jev-1.13-free","state":null,"questions":{"q":{"type":"noul"}}}`},
		{name: "no questions", raw: `{"model":"opencode/jev-1.13-free","state":"x"}`},
		{name: "an empty questions map", raw: `{"model":"opencode/jev-1.13-free","state":"x","questions":{}}`},
		{name: "an array for questions", raw: `{"model":"opencode/jev-1.13-free","state":"x","questions":[]}`},
		{name: "no model", raw: `{"state":"x","questions":{"q":{"type":"noul"}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := schema.DecodeSystemOneRequest([]byte(tc.raw))
			if err == nil {
				err = req.Validate()
			}
			if err == nil {
				t.Fatal("want a refusal for an unusable body")
			}
			if code := dataplane.AsError(err).Code; code != dataplane.CodeValidation {
				t.Fatalf("code = %q, want %q", code, dataplane.CodeValidation)
			}
		})
	}
}

// TestSystemOne_RefusesACombo pins the same rule the embeddings route keeps: a
// combo is an ordered list for chat failover, so honouring its first member would
// decide with a model the client did not name.
func TestSystemOne_RefusesACombo(t *testing.T) {
	usage, logs := &stubUsageRecorder{}, &stubLogRecorder{}
	svc, err := NewSystemOneService(SystemOneServiceDeps{
		Resolver: stubModelResolver{resolution: dataplane.Resolution{
			Provider: systemOneProvider(), Combo: systemOneCombo(t),
		}},
		Router: &stubMediaRouter{}, Caller: &stubMediaCaller{},
		Usage: usage, Logs: logs,
	})
	if err != nil {
		t.Fatalf("building the decision service: %v", err)
	}
	_, err = svc.Decide(context.Background(), decisionRequest(t), "key-1")
	if err == nil {
		t.Fatal("Decide() = nil error, want the combo refused")
	}
	if !strings.Contains(err.Error(), "combo") {
		t.Fatalf("error = %v, want it to name the combo", err)
	}
}

// TestSystemOne_TargetRefusesANonAbsoluteURL pins the URL rule directly, because
// a relative base would build a request to nowhere and the failure would surface
// as a transport error rather than as the configuration mistake it is.
func TestSystemOne_TargetRefusesANonAbsoluteURL(t *testing.T) {
	endpoint, err := domain.NewUpstreamEndpoint("ep", "opencode", "p", domain.UpstreamAuthNone, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("building the endpoint: %v", err)
	}
	selection := dataplane.Selection{Endpoint: endpoint, Credential: provider.NoCredential("ep")}

	cases := []struct {
		name string
		base string
	}{
		{name: "empty", base: "  "},
		{name: "relative", base: "/zen/v1/systemone"},
		{name: "a bare host", base: "opencode.ai/zen/v1/systemone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := systemOneTarget(&registry.SystemOneConfig{BaseURL: tc.base}, selection); err == nil {
				t.Fatalf("systemOneTarget(%q) = nil error, want a refusal", tc.base)
			}
		})
	}
}

// systemOneCombo builds a stored combo whose first member is the decision model,
// so the refusal case reads the same aggregate the chat plane would.
func systemOneCombo(t *testing.T) domain.Combo {
	t.Helper()
	model, err := domain.NewComboModel("opencode/jev-1.13-free", 1)
	if err != nil {
		t.Fatalf("NewComboModel() error = %v", err)
	}
	combo, err := domain.NewCombo("cmb_decision", "daily", domain.ComboFallback, 0, "", []domain.ComboModel{model}, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	return combo
}
