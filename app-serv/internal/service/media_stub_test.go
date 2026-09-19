// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_stub_test.go
// @for       The caller, router, and service fixture the §7.10 media call tests
//
//	drive.
//
// @uses      internal/dataplane, internal/domain, internal/provider, context,
//
//	testing, time.
//
// @reason    The media call service talks to the outside world through three
//
//	seams — the media caller, the router port, and the registry index — so
//	all three are doubles here and each test is about one rule. The
//	registry entries they read live in media_call_entries_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// stubMediaCaller records every outbound request and answers with one canned
// response, so a test can assert both what was sent and what came back.
type stubMediaCaller struct {
	requests []dataplane.MediaRequest
	answer   dataplane.MediaResponse
	failure  error
}

func (c *stubMediaCaller) Do(_ context.Context, request dataplane.MediaRequest) (dataplane.MediaResponse, error) {
	c.requests = append(c.requests, request)
	if c.failure != nil {
		return dataplane.MediaResponse{}, c.failure
	}
	return c.answer, nil
}

// stubMediaRouter answers selection with one fixed endpoint and records the
// health reports a call produces.
type stubMediaRouter struct {
	failure    error
	credential provider.Credential
	successes  int
	failures   []string
}

func (r *stubMediaRouter) Select(_ context.Context, providerID string) (dataplane.Selection, error) {
	if r.failure != nil {
		return dataplane.Selection{}, r.failure
	}
	endpoint, err := domain.NewUpstreamEndpoint("ep-"+providerID, providerID, "primary",
		domain.UpstreamAuthAPIKey, 1, time.Now().UTC())
	if err != nil {
		return dataplane.Selection{}, err
	}
	return dataplane.Selection{Endpoint: endpoint, Credential: r.credential}, nil
}

func (r *stubMediaRouter) RecordSuccess(context.Context, dataplane.Selection) error {
	r.successes++
	return nil
}

func (r *stubMediaRouter) RecordFailure(_ context.Context, _ dataplane.Selection, reason string) error {
	r.failures = append(r.failures, reason)
	return nil
}

// mediaCallFixture builds the service over the doubles. The registry entries it
// reads live in media_call_entries_test.go.
func mediaCallFixture(t *testing.T) (*MediaCallService, *stubMediaCaller, *stubMediaRouter) {
	t.Helper()
	caller := &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}}
	router := &stubMediaRouter{}
	svc, err := NewMediaCallService(MediaCallServiceDeps{
		Index:  &mediaIndex{entries: mediaCallEntries()},
		Router: router,
		Caller: caller,
	})
	if err != nil {
		t.Fatalf("NewMediaCallService() error = %v", err)
	}
	return svc, caller, router
}
