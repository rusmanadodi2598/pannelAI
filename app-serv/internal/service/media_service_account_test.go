// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_service_account_test.go
// @for       The service-account credential seam: the mint a service_account kind runs before its call, and the refusals that keep the stored JSON from being presented as a bearer.
// @uses      internal/dataplane, internal/domain, internal/provider, internal/schema, context, errors, strings, testing.
// @reason    A service-account kind stores the vendor's JSON like a key but authenticates with a short-lived token, so the boundary between exchanging it and presenting it is where a leak would hide: a call that skipped the mint would send a private key to the video upstream. The seam's absence and its failure are pinned for the same reason.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// vertexServiceAccountJSON is the credential an operator pastes. The minter
// reads it whole, so the tests only need to recognise the exact bytes.
const vertexServiceAccountJSON = `{"type":"service_account","client_email":"veo@pannelai.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASC\n-----END PRIVATE KEY-----\n"}`

// stubVertexToken records the credentials it was asked to exchange.
type stubVertexToken struct {
	credentials []string
	token       string
	failure     error
}

func (s *stubVertexToken) AccessToken(_ context.Context, serviceAccountJSON string) (string, error) {
	s.credentials = append(s.credentials, serviceAccountJSON)
	if s.failure != nil {
		return "", s.failure
	}
	return s.token, nil
}

// vertexVideoFixture wires the doubles for a service_account video call: the
// router hands back a service_account endpoint carrying the project and
// location, and the account's credential is the stored JSON.
func vertexVideoFixture(t *testing.T, tokens provider.VertexToken) (*MediaCallService, *stubMediaCaller, *stubMediaRouter) {
	t.Helper()
	svc, caller, router := mediaCallFixtureWithTokens(t, tokens)
	router.credential = provider.StaticKey("ep-vertex", "key-1", vertexServiceAccountJSON)
	router.authType = domain.UpstreamAuthServiceAccount
	router.serviceAccount = domain.ServiceAccountRef{ProjectID: "demo-project", Location: "us-central1"}
	return svc, caller, router
}

// vertexCreate runs one vertex video creation through the service.
func vertexCreate(svc *MediaCallService) error {
	_, _, err := svc.GenerateVideoJob(context.Background(), schema.VideoRequest{
		Model: "vertex/" + vertexModel, Prompt: "a paper boat",
	}, []byte(`{"model":"vertex/`+vertexModel+`","prompt":"a paper boat"}`), "")
	return err
}

// TestMediaCallService_MintsTheServiceAccountBearer pins the seam: a kind
// declaring auth_type service_account stores the vendor's JSON like a key, and
// the outbound call presents the minted bearer rather than the JSON.
func TestMediaCallService_MintsTheServiceAccountBearer(t *testing.T) {
	minter := &stubVertexToken{token: "ya29.minted"}
	svc, caller, _ := vertexVideoFixture(t, minter)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"name":"` + vertexOperation + `","done":false}`)}

	if err := vertexCreate(svc); err != nil {
		t.Fatalf("GenerateVideoJob() error = %v", err)
	}
	if len(minter.credentials) != 1 || minter.credentials[0] != vertexServiceAccountJSON {
		t.Fatalf("minted from %v, want the stored service-account JSON", minter.credentials)
	}
	if got := caller.requests[0].Headers["Authorization"]; got != "Bearer ya29.minted" {
		t.Fatalf("Authorization = %q, want the minted bearer", got)
	}
	if strings.Contains(string(caller.requests[0].Body), "private_key") {
		t.Fatal("the outbound request body carries the service-account JSON")
	}
}

// TestMediaCallService_RefusesAServiceAccountCallWithoutAMinter pins the other
// half of the seam: a deployment without the minter refuses the call rather
// than presenting the stored JSON as a bearer.
func TestMediaCallService_RefusesAServiceAccountCallWithoutAMinter(t *testing.T) {
	svc, caller, _ := vertexVideoFixture(t, nil)

	err := vertexCreate(svc)
	if err == nil || !strings.Contains(err.Error(), "minter") {
		t.Fatalf("error = %v, want the missing-minter refusal", err)
	}
	if len(caller.requests) != 0 {
		t.Fatalf("outbound calls = %d, want none", len(caller.requests))
	}
}

// TestMediaCallService_RefusesWhenTheMintFails pins that a refused exchange
// surfaces the vendor's own reason and still sends nothing upstream.
func TestMediaCallService_RefusesWhenTheMintFails(t *testing.T) {
	minter := &stubVertexToken{failure: errors.New("the service-account key was not accepted: Invalid JWT Signature.")}
	svc, caller, _ := vertexVideoFixture(t, minter)

	err := vertexCreate(svc)
	if err == nil || !strings.Contains(err.Error(), "Invalid JWT Signature") {
		t.Fatalf("error = %v, want the exchange's own reason", err)
	}
	if len(caller.requests) != 0 {
		t.Fatalf("outbound calls = %d, want none", len(caller.requests))
	}
}

// TestMediaCallService_ApiKeyCallsNeverConsultTheMinter pins the seam's
// boundary: only a service_account kind is exchanged.
func TestMediaCallService_ApiKeyCallsNeverConsultTheMinter(t *testing.T) {
	minter := &stubVertexToken{token: "unused"}
	svc, caller, _ := mediaCallFixtureWithTokens(t, minter)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"request_id":"req-1"}`)}

	if _, _, err := svc.GenerateVideoJob(context.Background(), schema.VideoRequest{
		Model: "xai/grok-imagine-video", Prompt: "a paper boat",
	}, []byte(`{"model":"xai/grok-imagine-video","prompt":"a paper boat"}`), ""); err != nil {
		t.Fatalf("GenerateVideoJob() error = %v", err)
	}
	if len(minter.credentials) != 0 {
		t.Fatalf("mints = %v, want none for an api_key kind", minter.credentials)
	}
}
