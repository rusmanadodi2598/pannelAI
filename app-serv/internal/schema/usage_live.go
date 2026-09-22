// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/usage_live.go
// @for       The live Usage frame: the three facts a period window cannot carry,
//
//	in the shape the panel parses.
//
// @uses      internal/domain, time.
// @reason    SPEC-UI-001 §6.5 fixes the frame's contents (the requests in flight
//
//	now, the requests that just finished, and the provider the gateway
//	last reported an error for) and app-ui already parses them with a Zod
//	schema, so the member names are a contract between two applications
//	rather than a local choice. Keeping the mapping here means the
//	handler writes bytes and nothing else, and the panel's own rule (a
//	key the frame omits is not a statement that nothing is happening)
//	is satisfied by construction: every member is always present.
//
//	Both lists are always rendered as arrays, never as `null`. Go marshals
//	a nil slice as `null`, and the panel normalizes that, but a frame
//	that says "nothing is running" and a frame that says nothing are
//	different facts to a reader diffing the wire, so the empty case is
//	spelled one way.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-22
package schema

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// UsageLiveActive is one request the gateway is routing right now.
//
// `started_at` is present because the panel's staleness guard reads it: without
// it the panel could not tell a request still running from one whose marker the
// gateway never cleared, and it would light a node forever.
type UsageLiveActive struct {
	ProviderID string `json:"provider_id"`
	EndpointID string `json:"endpoint_id"`
	Model      string `json:"model"`
	StartedAt  string `json:"started_at"`
}

// UsageLiveRecent is one request that finished recently. The list is bounded by
// the read, so the panel renders it rather than paging it.
type UsageLiveRecent struct {
	RequestID  string `json:"request_id"`
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	TS         string `json:"ts"`
	Status     string `json:"status"`
	TokensIn   int64  `json:"tokens_in"`
	TokensOut  int64  `json:"tokens_out"`
	ErrorCode  string `json:"error_code"`
}

// UsageLiveFrame is one `data:` payload of the live stream. It is full state for
// the three facts it carries, not a delta, so a client that missed a frame
// recovers on the next one.
type UsageLiveFrame struct {
	// Active is the in-flight set, oldest first, so the longest-running request
	// is the first row.
	Active []UsageLiveActive `json:"active"`
	// Recent is the requests that just finished, newest first, bounded by the
	// service's read.
	Recent []UsageLiveRecent `json:"recent"`
	// ErrorProvider is the provider the gateway last reported an error for, or
	// an empty string when the last attempt to reach a provider succeeded.
	ErrorProvider string `json:"error_provider"`
}

// UsageLiveFrameFrom maps the gateway's state onto the wire frame.
//
// The order of both lists is preserved rather than re-sorted: the caller's read
// is what bounded and ordered them, and a second ordering rule here would be a
// second answer to "which request is first" that could disagree with the query.
func UsageLiveFrameFrom(markers []domain.ActiveRequest, recent []domain.UsageRecord, errorProvider string) UsageLiveFrame {
	frame := UsageLiveFrame{
		Active:        make([]UsageLiveActive, 0, len(markers)),
		Recent:        make([]UsageLiveRecent, 0, len(recent)),
		ErrorProvider: errorProvider,
	}
	for _, marker := range markers {
		frame.Active = append(frame.Active, UsageLiveActive{
			ProviderID: marker.ProviderID,
			EndpointID: marker.EndpointID,
			Model:      marker.Model,
			StartedAt:  Timestamp(marker.StartedAt),
		})
	}
	for _, record := range recent {
		frame.Recent = append(frame.Recent, UsageLiveRecent{
			RequestID:  record.RequestID(),
			ProviderID: record.ProviderID(),
			Model:      record.Model(),
			TS:         Timestamp(record.TS()),
			Status:     record.Status(),
			TokensIn:   record.TokensIn(),
			TokensOut:  record.TokensOut(),
			ErrorCode:  record.ErrorCode(),
		})
	}
	return frame
}
