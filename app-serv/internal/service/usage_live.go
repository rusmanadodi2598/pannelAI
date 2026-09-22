// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_live.go
// @for       The live Usage frame: the in-flight set, the requests that just
//
//	finished, and the provider last reported in error.
//
// @uses      internal/domain, internal/repository, internal/schema, context, time.
// @reason    SPEC-UI-001 §6.5 fixes what the live stream carries and forbids the
//
//	panel from computing what it cannot cite, so the three facts are
//	assembled here from two sources the gateway already owns: the
//	in-flight marker set it writes while calls run, and the durable
//	usage rows it writes when they finish. Nothing on this path
//	aggregates, and the frame has no field for a total, which is what
//	keeps the stream structurally unable to restate the REST reads.
//
//	Both halves are bounded. The recent read takes a window and a row
//	limit, so the frame is a fixed size no matter how busy the gateway
//	is (§1.7: no unbounded query in a request path), and the in-flight
//	read is bounded by the store.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// The live frame's bounds. Each is a constant rather than a setting because the
// frame is rendered, not paged, and a client that asked for a different size
// would be asking for a different screen.
const (
	// UsageLiveRecentWindow is how far back the finished-request list reaches.
	// It matches the reference fork's own ring window closely enough to show the
	// same recent traffic without reading a table the panel cannot render.
	UsageLiveRecentWindow = 5 * time.Minute
	// UsageLiveRecentLimit is the most finished requests one frame carries. The
	// reference keeps twenty, and the panel renders the list rather than paging
	// it, so the bound is what keeps the frame small on a busy gateway.
	UsageLiveRecentLimit = 20
	// UsageLiveErrorWindow is how long a failure keeps naming its provider.
	// It is deliberately much shorter than the recent window: the panel draws
	// that provider's node in the error state, which is a claim about what is
	// wrong now, not a record of what went wrong earlier.
	UsageLiveErrorWindow = 10 * time.Second
	// usageLiveActiveLimit bounds the in-flight list for the same reason as the
	// recent limit: a gateway under a burst must not send an unbounded frame.
	usageLiveActiveLimit = 50
)

// UsageLiveService assembles the live Usage frame (SPEC-API-001 §7.12).
type UsageLiveService struct {
	active repository.ActiveRequestStore
	usage  repository.UsageRecordRepository
	clock  func() time.Time
}

// UsageLiveServiceDeps holds the service's collaborators.
//
// Usage is required: without the durable rows there is no recent half, and a
// service that booted anyway would answer every connection with a frame it
// could not fill. Active is optional, and a deployment that wired no Redis
// reports an empty in-flight set rather than refusing the route.
type UsageLiveServiceDeps struct {
	Active repository.ActiveRequestStore
	Usage  repository.UsageRecordRepository
}

// NewUsageLiveService validates deps and returns a ready service.
func NewUsageLiveService(deps UsageLiveServiceDeps) (*UsageLiveService, error) {
	if deps.Usage == nil {
		return nil, domain.NewValidationError("usage repository is required")
	}
	return &UsageLiveService{active: deps.Active, usage: deps.Usage, clock: time.Now}, nil
}

// SetClock replaces the service's clock so a test can pin the read window
// without sleeping. It is not part of the production path.
func (s *UsageLiveService) SetClock(clock func() time.Time) {
	if s != nil && clock != nil {
		s.clock = clock
	}
}

// Snapshot reads the three facts once, at one instant, so every value in the
// frame describes the same moment.
//
// A failure in either read is returned rather than answered with an empty half.
// An empty in-flight set is a claim that nothing is routing, and a failed read
// cannot support that claim: the stream states the failure instead (R-36).
func (s *UsageLiveService) Snapshot(ctx context.Context) (schema.UsageLiveFrame, error) {
	now := s.clock()
	markers, err := s.activeMarkers(ctx, now)
	if err != nil {
		return schema.UsageLiveFrame{}, err
	}
	recent, err := s.recentRecords(ctx, now)
	if err != nil {
		return schema.UsageLiveFrame{}, err
	}
	return schema.UsageLiveFrameFrom(markers, recent, errorProviderOf(recent, now)), nil
}

// activeMarkers reads the bounded in-flight set. A nil store reports none, which
// is the honest answer for a deployment that wired no broker.
func (s *UsageLiveService) activeMarkers(ctx context.Context, now time.Time) ([]domain.ActiveRequest, error) {
	if s.active == nil {
		return nil, nil
	}
	return s.active.Active(ctx, now, usageLiveActiveLimit)
}

// recentRecords reads the bounded finished-request page, newest first.
func (s *UsageLiveService) recentRecords(ctx context.Context, now time.Time) ([]domain.UsageRecord, error) {
	filter := domain.NewUsageFilter(domain.UsageFilterInput{
		From: timePtr(now.Add(-UsageLiveRecentWindow)),
		To:   timePtr(now),
	}, now)
	records, _, err := s.usage.List(ctx, filter, repository.PageQuery{Page: 1, PerPage: UsageLiveRecentLimit})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// errorProviderOf names the provider of the newest failed request inside its own
// short window, or an empty string when there is none.
//
// It is derived from the same bounded read the frame already carries rather than
// from a second key, so the provider the node marks in error and the failure the
// list shows can never disagree about which request they came from.
func errorProviderOf(recent []domain.UsageRecord, now time.Time) string {
	cutoff := now.Add(-UsageLiveErrorWindow)
	for _, record := range recent {
		if !record.TS().After(cutoff) {
			// A failure exactly at the window is already outside it, and the read
			// is newest first, so the first record that is not inside the window
			// ends the search: nothing after it can be newer.
			return ""
		}
		if record.UsageStatus() == domain.UsageStatusError {
			return record.ProviderID()
		}
	}
	return ""
}

// timePtr returns a pointer to a copy of value, which is the shape the filter
// input uses to tell "not asked for" from "asked for the zero instant".
func timePtr(value time.Time) *time.Time { return &value }
