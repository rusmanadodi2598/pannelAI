// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_live_tracker.go
// @for       Recording one call as in flight for as long as it is running, at
//
//	each plane's outbound seam.
//
// @uses      internal/domain, internal/repository, context, log/slog, time.
// @reason    SPEC-UI-001 §6.5 makes the drawing's active node mean "a request is
//
//	being routed now", and only the gateway knows that interval. The
//	tracker is the write half of the pair: `Begin` records a marker
//	before the outbound call and returns the release that removes it
//	after, so the two cannot be separated by a caller that forgets the
//	second half.
//
//	It is one type applied at three seams rather than one central hook,
//	because the gateway has three outbound planes (chat through the
//	engine, media, and embeddings) and no single function every call
//	passes through. The precedent is the quota counter and the accounting
//	recorder, which are applied the same way for the same reason: the
//	alternative would put accounting fields on the transports' own call
//	structs, where they do not belong (AGENTS.md §1.5).
//
//	Nothing here can fail a request. A store write is bookkeeping: the
//	client's answer does not depend on the drawing, so a failure is
//	logged and the call proceeds, the same rule every other accounting
//	write in this service follows.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ActiveRequestTracker records in-flight markers for the calls this process is
// routing. A nil store makes it a no-op, which is the documented behaviour for a
// deployment that wired no Redis.
type ActiveRequestTracker struct {
	store     repository.ActiveRequestStore
	requestID RequestIDReader
	logger    *slog.Logger
	clock     func() time.Time
}

// NewActiveRequestTracker binds the tracker to its store and request-id reader.
// The store and the reader are both optional: without a store nothing is
// recorded, and without a reader a marker gets a fresh id of its own.
func NewActiveRequestTracker(store repository.ActiveRequestStore, requestID RequestIDReader, logger *slog.Logger) *ActiveRequestTracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ActiveRequestTracker{store: store, requestID: requestID, logger: logger, clock: time.Now}
}

// SetClock replaces the tracker's clock so a test can pin the start instant
// without sleeping. It is not part of the production path.
func (t *ActiveRequestTracker) SetClock(clock func() time.Time) {
	if t != nil && clock != nil {
		t.clock = clock
	}
}

// markActiveRequest opens one marker through the seam and returns its release.
//
// It is the nil-safe form the media and embeddings planes share, because a typed
// nil inside an interface still panics when its method is called: both services
// hold the seam as an interface, so a deployment that wired no tracker would
// reach a nil receiver rather than the no-op the seam promises. The engine keeps
// its own copy of this rule on its side of the package boundary.
func markActiveRequest(ctx context.Context, active dataplane.ActiveRequests, providerID, endpointID, model string) func() {
	if active == nil {
		return func() {}
	}
	return active.Begin(ctx, providerID, endpointID, model)
}

// Begin records one call as in flight and returns the function that releases it.
//
// The release is safe to call more than once, so a caller may both defer it and
// call it explicitly on the path that ends the call, which is what the three
// planes do: the marker has to disappear when the outbound call returns, not
// only when the handler does.
//
// A call that resolved no provider records nothing: the marker's whole purpose
// is to light a provider's node, and a marker with no provider would light
// nothing while still occupying the set.
func (t *ActiveRequestTracker) Begin(ctx context.Context, providerID, endpointID, model string) func() {
	if t == nil || t.store == nil || providerID == "" {
		return func() {}
	}
	now := t.clock()
	requestID := ""
	if t.requestID != nil {
		requestID = t.requestID(ctx)
	}
	if requestID == "" {
		requestID = domain.NewULID(now)
	}
	marker, err := domain.NewActiveRequest(requestID, providerID, endpointID, model, now)
	if err != nil {
		// A refused marker is a caller bug rather than a runtime condition: the
		// only refusals are a missing request id or provider, and this function
		// has already supplied both. It is logged rather than returned because
		// the call it describes must still run.
		t.logger.Error("an active request marker was refused by its own constructor",
			"provider_id", providerID, "error", err)
		return func() {}
	}
	if err := t.store.Start(ctx, marker); err != nil {
		// reason: the marker is a view, not the record. A request must not fail
		// because the drawing could not be told about it.
		t.logger.Warn("recording an active request failed",
			"provider_id", providerID, "request_id", marker.RequestID, "error", err)
	}
	var once bool
	return func() {
		if once {
			return
		}
		once = true
		// reason: releasing the marker is bookkeeping, and the staleness window
		// is the backstop; a failed release retries on no path and does not
		// deserve an error the caller cannot act on.
		if err := t.store.Finish(ctx, marker); err != nil {
			t.logger.Warn("releasing an active request failed",
				"provider_id", providerID, "request_id", marker.RequestID, "error", err)
		}
	}
}
