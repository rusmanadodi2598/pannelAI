// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings_active_test.go
// @for       The in-flight marker at the embeddings outbound seam.
//
// @uses      context, testing, internal/dataplane, internal/domain, internal/provider.
// @reason    The media and embeddings planes reach an upstream through their own
//
//	services rather than the chat engine, so each one has to open and
//	close its own marker (SPEC-UI-001 §6.5: the drawing shows what is
//	routing now, whichever plane is routing it). The failure paths are
//	the ones that matter: a marker left open on a rejected call keeps a
//	node lit for a request that ended.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// TestEmbeddingsPerform_MarksTheProviderWhileItRuns pins the same interval on
// the embeddings plane, which reaches an upstream through its own service rather
// than through the media call service.
func TestEmbeddingsPerform_MarksTheProviderWhileItRuns(t *testing.T) {
	cases := []struct {
		name     string
		answer   dataplane.MediaResponse
		failure  error
		wantFail bool
	}{
		{name: "the call is served", answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"data":[]}`)}},
		{name: "the upstream is unreachable", failure: dataplane.InternalError("dial failed", nil), wantFail: true},
		{name: "the upstream rejects the request", answer: dataplane.MediaResponse{Status: 400, Body: []byte(`{"error":"bad"}`)}, wantFail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			active := &activeRecorderDouble{}
			caller := &stubMediaCaller{answer: tc.answer, failure: tc.failure}
			svc, err := NewEmbeddingsService(EmbeddingsServiceDeps{
				Resolver:       stubModelResolver{},
				Router:         &stubMediaRouter{},
				Caller:         caller,
				ActiveRequests: active,
			})
			if err != nil {
				t.Fatalf("NewEmbeddingsService() error = %v", err)
			}
			outcome := dataplane.Outcome{ProviderID: "openai", EndpointID: "ep-openai", Model: "text-embedding-3"}
			endpoint, err := domain.NewUpstreamEndpoint("ep-openai", "openai", "primary",
				domain.UpstreamAuthAPIKey, 1, time.Now().UTC())
			if err != nil {
				t.Fatalf("building the endpoint: %v", err)
			}
			selection := dataplane.Selection{Endpoint: endpoint, Credential: provider.Credential{}}

			_, err = svc.perform(context.Background(), dataplane.MediaRequest{Method: "POST", Body: []byte(`{}`)},
				selection, outcome, "gky_1")
			if tc.wantFail && err == nil {
				t.Fatal("perform() = nil, want a failure")
			}
			if !tc.wantFail && err != nil {
				t.Fatalf("perform() = %v, want nil", err)
			}

			begun, released, stillOpen := active.snapshot()
			if len(begun) != 1 || begun[0] != "openai" {
				t.Fatalf("markers begun = %v, want one for openai", begun)
			}
			if released != 1 || stillOpen != 0 {
				t.Fatalf("released = %d and still open = %d, want 1 and 0", released, stillOpen)
			}
		})
	}
}
