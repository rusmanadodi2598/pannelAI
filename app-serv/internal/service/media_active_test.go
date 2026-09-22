// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_active_test.go
// @for       The in-flight marker at the media and embeddings outbound seams:
//
//	one provider recorded while its call runs, released on every exit.
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
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// activeRecorderDouble observes the marker's interval for the media planes.
type activeRecorderDouble struct {
	mu        sync.Mutex
	begun     []string
	released  int
	stillOpen int
}

func (r *activeRecorderDouble) Begin(_ context.Context, providerID, _, _ string) func() {
	r.mu.Lock()
	r.begun = append(r.begun, providerID)
	r.stillOpen++
	r.mu.Unlock()

	var once bool
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if once {
			return
		}
		once = true
		r.released++
		r.stillOpen--
	}
}

func (r *activeRecorderDouble) snapshot() (begun []string, released, stillOpen int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.begun...), r.released, r.stillOpen
}

// mediaServiceWithActive builds the media call service over the shared doubles
// plus the marker seam under test.
func mediaServiceWithActive(t *testing.T, caller *stubMediaCaller, active dataplane.ActiveRequests) *MediaCallService {
	t.Helper()
	svc, err := NewMediaCallService(MediaCallServiceDeps{
		Index:          &mediaIndex{entries: mediaCallEntries()},
		Router:         &stubMediaRouter{},
		Caller:         caller,
		ActiveRequests: active,
	})
	if err != nil {
		t.Fatalf("NewMediaCallService() error = %v", err)
	}
	return svc
}

// activeMediaCall builds one resolved media call the service can perform.
func activeMediaCall(t *testing.T, providerID string) MediaCall {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint("ep-"+providerID, providerID, "primary",
		domain.UpstreamAuthAPIKey, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("building the endpoint: %v", err)
	}
	return MediaCall{
		ProviderID: providerID, UpstreamModel: "tts-1",
		BaseURL: "http://upstream.test", Target: "http://upstream.test/x",
		Selection: dataplane.Selection{Endpoint: endpoint, Credential: provider.Credential{}},
	}
}

// TestMediaPerform_MarksTheProviderWhileItRuns pins the media plane's marker: it
// opens before the outbound call and closes before Perform returns.
func TestMediaPerform_MarksTheProviderWhileItRuns(t *testing.T) {
	active := &activeRecorderDouble{}
	caller := &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}}
	svc := mediaServiceWithActive(t, caller, active)

	if _, err := svc.Perform(context.Background(), activeMediaCall(t, "openai"),
		dataplane.MediaRequest{Method: "POST", Body: []byte(`{}`)}, "gky_1", nil); err != nil {
		t.Fatalf("Perform() = %v, want nil", err)
	}

	begun, released, stillOpen := active.snapshot()
	if len(begun) != 1 || begun[0] != "openai" {
		t.Fatalf("markers begun = %v, want one for openai", begun)
	}
	if released != 1 || stillOpen != 0 {
		t.Fatalf("released = %d and still open = %d, want 1 and 0", released, stillOpen)
	}
}

// TestMediaPerform_ReleasesTheMarkerOnEveryExit pins the three endings a media
// call can have: unreachable, rejected, and served. Every one of them must
// release, because the marker is an interval rather than an event.
func TestMediaPerform_ReleasesTheMarkerOnEveryExit(t *testing.T) {
	cases := []struct {
		name     string
		answer   dataplane.MediaResponse
		failure  error
		wantFail bool
	}{
		{name: "the upstream is unreachable", failure: dataplane.InternalError("dial failed", nil), wantFail: true},
		{name: "the upstream rejects the request", answer: dataplane.MediaResponse{Status: 429, Body: []byte(`{"error":"rate limited"}`)}, wantFail: true},
		{name: "the upstream serves the call", answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			active := &activeRecorderDouble{}
			caller := &stubMediaCaller{answer: tc.answer, failure: tc.failure}
			svc := mediaServiceWithActive(t, caller, active)

			_, err := svc.Perform(context.Background(), activeMediaCall(t, "openai"),
				dataplane.MediaRequest{Method: "POST", Body: []byte(`{}`)}, "gky_1", nil)
			if tc.wantFail && err == nil {
				t.Fatal("Perform() = nil, want a failure")
			}
			if !tc.wantFail && err != nil {
				t.Fatalf("Perform() = %v, want nil", err)
			}

			begun, released, stillOpen := active.snapshot()
			if len(begun) != 1 {
				t.Fatalf("markers begun = %v, want one", begun)
			}
			if released != 1 || stillOpen != 0 {
				t.Fatalf("released = %d and still open = %d, want 1 and 0: every exit must release", released, stillOpen)
			}
		})
	}
}

// TestMediaPerform_WithNoTrackerIsANoOp pins the unwired deployment: the call
// still runs and no marker is attempted.
func TestMediaPerform_WithNoTrackerIsANoOp(t *testing.T) {
	caller := &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}}
	svc := mediaServiceWithActive(t, caller, nil)
	if _, err := svc.Perform(context.Background(), activeMediaCall(t, "openai"),
		dataplane.MediaRequest{Method: "POST", Body: []byte(`{}`)}, "gky_1", nil); err != nil {
		t.Fatalf("Perform() with no tracker = %v, want nil", err)
	}
}
