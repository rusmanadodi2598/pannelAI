// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_outcome_test.go
// @for       Tests for the health decision a finished leg makes: a stream the caller abandoned records nothing, a answered one still clears the park.
// @uses      context, testing, time, internal/domain.
// @reason    A client closing a tab mid-stream stops the upstream read quietly, and the relay read that quiet stop as a served call. The result was that any request could erase a park a real upstream failure had just earned, putting a broken credential straight back into rotation. The inverse matters equally: an answer that really did land must still clear the park, or a recovered credential stays buried.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// parkedKeyEngine selects a credential and parks it with one refused call, so a
// test can watch whether the next leg's accounting keeps or clears that park.
func parkedKeyEngine(t *testing.T) (*Engine, *memEndpointRepo, Selection) {
	t.Helper()
	repo := twoIdleKeyRepo(t)
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection, err := selector.Select(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if _, err := selector.RecordFailure(context.Background(), selection, "the credential was refused",
		domain.KeyFailureAuth, domain.RestDirective{}); err != nil {
		t.Fatalf("RecordFailure() error = %v", err)
	}
	parked := repo.health[selection.Key.ID()]
	if parked.RateLimitedUntil() == nil {
		t.Fatal("the fixture did not park the key")
	}
	return &Engine{selector: selector, logger: nil}, repo, selection
}

// TestEngine_RecordServedOutcomeAbandonedStreamKeepsThePark is the defect: the
// caller stopped listening, so nothing about the credential was proven, and the
// park another request earned must survive.
func TestEngine_RecordServedOutcomeAbandonedStreamKeepsThePark(t *testing.T) {
	engine, repo, selection := parkedKeyEngine(t)
	writes := len(repo.outcome)

	engine.recordServedOutcome(context.Background(), &Outcome{StreamAborted: true}, selection)

	if len(repo.outcome) != writes {
		t.Fatalf("outcome writes = %d, want %d: an abandoned stream records nothing", len(repo.outcome), writes)
	}
	if repo.health[selection.Key.ID()].RateLimitedUntil() == nil {
		t.Fatal("the park was cleared by a call the client never received")
	}
}

// TestEngine_RecordServedOutcomeAnsweredCallClearsThePark is the other half: a
// credential that really did answer comes back into rotation on the next selection.
func TestEngine_RecordServedOutcomeAnsweredCallClearsThePark(t *testing.T) {
	engine, repo, selection := parkedKeyEngine(t)

	engine.recordServedOutcome(context.Background(), &Outcome{}, selection)

	if served := repo.health[selection.Key.ID()]; served.RateLimitedUntil() != nil {
		t.Fatalf("park = %v, want released by an answered call", served.RateLimitedUntil())
	}
	if served := repo.health[selection.Key.ID()]; served.ConsecutiveErrors() != 0 {
		t.Fatalf("error run = %d, want 0", served.ConsecutiveErrors())
	}
}

func TestEngine_RecordFailureClientCancellationWritesNothing(t *testing.T) {
	engine, repo, selection := parkedKeyEngine(t)
	_, before, _ := repo.outcome[selection.Endpoint.ID()].LastError()

	engine.recordFailure(context.Background(), selection, context.Canceled)

	_, after, _ := repo.outcome[selection.Endpoint.ID()].LastError()
	if after != before {
		t.Fatalf("last error = %q, want the hang-up to write nothing", after)
	}
	if before != "the credential was refused" {
		t.Fatalf("fixture last error = %q, want the park's reason on record first", before)
	}
}

func TestEngine_RecordFailureUpstreamFailureStillWrites(t *testing.T) {
	engine, repo, selection := parkedKeyEngine(t)

	// A cancelled context that wrapped a real upstream failure is still an
	// upstream failure: only a cancellation the read itself returned is silence.
	engine.recordFailure(context.Background(), selection,
		&UpstreamError{Status: 500, Message: "the upstream broke"})

	if _, got, _ := repo.outcome[selection.Endpoint.ID()].LastError(); got != "the upstream broke" {
		t.Fatalf("last error = %q, want the upstream's reason recorded", got)
	}
}

func TestIsClientCancellation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "a bare cancellation", err: context.Canceled, want: true},
		{name: "a cancellation wrapped in the read", err: errors.New("reading the stream: " + context.Canceled.Error()), want: false},
		{name: "a wrapped cancellation is one", err: wrapCancel(), want: true},
		{name: "a deadline is not a hang-up", err: context.DeadlineExceeded},
		{name: "an upstream refusal is not one", err: &UpstreamError{Status: 429}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isClientCancellation(tc.err); got != tc.want {
				t.Fatalf("isClientCancellation(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func wrapCancel() error { return errors.Join(errors.New("stream stopped"), context.Canceled) }
