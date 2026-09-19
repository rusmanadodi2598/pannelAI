// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_perform_test.go
// @for       How a media call's outcome reaches endpoint health.
// @uses      internal/dataplane, internal/domain, context, errors, testing.
// @reason    A served call, a rejection, and an unreachable upstream must land
//
//	in the same circuit state the chat plane reads, and the client must
//	see a failure the gateway can explain. Each case is a table row so a
//	new status can be added without rewriting a test. The resolution
//	rules live in media_call_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestMediaCallService_Perform pins how an answer and a failure reach endpoint
// health: a served call counts as a success, and a rejection or an unreachable
// upstream counts as a failure the client also sees.
func TestMediaCallService_Perform(t *testing.T) {
	cases := []struct {
		name          string
		answer        dataplane.MediaResponse
		failure       error
		wantCode      string
		wantSuccesses int
		wantFailures  int
	}{
		{
			name: "a served call", answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)},
			wantSuccesses: 1,
		},
		{
			name: "a rate-limited upstream", answer: dataplane.MediaResponse{Status: 429, Body: []byte(`{"error":{"message":"slow down"}}`)},
			wantCode: dataplane.CodeRateLimited, wantFailures: 1,
		},
		{
			name: "a rejected upstream", answer: dataplane.MediaResponse{Status: 400, Body: []byte(`{"message":"bad model"}`)},
			wantCode: dataplane.CodeUpstreamError, wantFailures: 1,
		},
		{
			name: "an unreachable upstream", failure: errors.New("connection refused"),
			wantCode: dataplane.CodeInternal, wantFailures: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			caller.answer = tc.answer
			caller.failure = tc.failure

			call, err := svc.Prepare(context.Background(), "openai/gpt-4o-mini-tts", domain.MediaKindTTS, nil)
			if err != nil {
				t.Fatalf("Prepare() error = %v", err)
			}
			_, err = svc.Perform(context.Background(), call, dataplane.MediaRequest{Body: []byte(`{}`)})
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("Perform() error = %v", err)
				}
			} else {
				failure := dataplane.AsError(err)
				if failure.Code != tc.wantCode {
					t.Fatalf("code = %s, want %s", failure.Code, tc.wantCode)
				}
			}
			if router.successes != tc.wantSuccesses {
				t.Fatalf("successes = %d, want %d", router.successes, tc.wantSuccesses)
			}
			if len(router.failures) != tc.wantFailures {
				t.Fatalf("failures = %v, want %d", router.failures, tc.wantFailures)
			}
		})
	}
}

// TestMediaCallService_PerformMergesHeaders pins that a route's own headers win
// over the target's, which is how a multipart body replaces the JSON content
// type without the service having to know about multipart.
func TestMediaCallService_PerformMergesHeaders(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	call, err := svc.Prepare(context.Background(), "openai/gpt-4o-mini-tts", domain.MediaKindTTS, nil)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	_, err = svc.Perform(context.Background(), call, dataplane.MediaRequest{
		Headers: map[string]string{"Content-Type": "multipart/form-data; boundary=x"},
	})
	if err != nil {
		t.Fatalf("Perform() error = %v", err)
	}
	if got := caller.requests[0].Headers["Content-Type"]; got != "multipart/form-data; boundary=x" {
		t.Fatalf("content type = %q, want the caller's", got)
	}
	if caller.requests[0].URL != call.Target {
		t.Fatalf("URL = %q, want the prepared target", caller.requests[0].URL)
	}
}

// TestNewMediaCallService_RequiresDeps pins the constructor's refusals, so a
// missing collaborator fails at boot rather than on the first request.
func TestNewMediaCallService_RequiresDeps(t *testing.T) {
	caller := &stubMediaCaller{}
	cases := []struct {
		name string
		deps MediaCallServiceDeps
	}{
		{name: "no index", deps: MediaCallServiceDeps{Router: &stubMediaRouter{}, Caller: caller}},
		{name: "no router", deps: MediaCallServiceDeps{Index: &mediaIndex{}, Caller: caller}},
		{name: "no caller", deps: MediaCallServiceDeps{Index: &mediaIndex{}, Router: &stubMediaRouter{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewMediaCallService(tc.deps); err == nil {
				t.Fatal("NewMediaCallService() accepted incomplete deps")
			}
		})
	}
}
