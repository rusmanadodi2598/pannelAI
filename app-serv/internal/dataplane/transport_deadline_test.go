// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_deadline_test.go
// @for       The deadline one outbound attempt runs under, per the client's shape
//
//	and the registry's override (draft 021, the fold's missing bound).
//
// @uses      testing, time, internal/registry.
// @reason    A client that asked for one body stayed unbounded when its provider
//
//	refused a non-streaming request and the gateway folded a stream
//	instead, because the deadline was chosen from the rewritten upstream
//	shape. The rule is a pure function of the two inputs, so it is pinned
//	here with no network (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package dataplane

import (
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestAttemptDeadline pins the rule: the client's shape decides, and a registry
// override only ever tightens or widens the non-streamed bound.
func TestAttemptDeadline(t *testing.T) {
	cases := []struct {
		name         string
		clientStream bool
		entry        registry.Provider
		want         time.Duration
	}{
		{
			name:         "a client that asked for a stream has no total cap",
			clientStream: true,
			want:         0,
		},
		{
			name:         "a client that asked for one body is bounded by the total timeout",
			clientStream: false,
			want:         TotalTimeout,
		},
		{
			name:         "the registry's override replaces the total timeout",
			clientStream: false,
			entry:        registry.Provider{Transport: registry.Transport{TimeoutMS: 45000}},
			want:         45 * time.Second,
		},
		{
			name:         "the override does not cap a streamed client",
			clientStream: true,
			entry:        registry.Provider{Transport: registry.Transport{TimeoutMS: 45000}},
			want:         0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := attemptDeadline(tc.clientStream, tc.entry); got != tc.want {
				t.Fatalf("attemptDeadline(%v) = %v, want %v", tc.clientStream, got, tc.want)
			}
		})
	}
}
