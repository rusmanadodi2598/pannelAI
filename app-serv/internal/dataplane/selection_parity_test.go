// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_parity_test.go
// @for       That an outcome is also accounted onto the endpoint's own connection-parity state, not only onto the key's circuit.
// @uses      context, testing, time, internal/domain.
// @reason    R17 of docs/DRAFT/042-CODE-REVIEW-FIXES.md found the domain's RecordUpstreamSuccess and RecordUpstreamError with no caller on the data plane, so the panel's last-error block stayed empty however the upstream behaved. The selector is the one place an outcome is known, so the parity write is pinned here beside the key circuit it accompanies.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package dataplane

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestSelector_ParityAccounting pins that an upstream outcome reaches the
// endpoint's parity state: a served call clears a recorded error and the use
// run, a transient failure and a rate limit are recorded under the wire's codes,
// a request-shaped failure writes nothing, and a keyless selection still reports
// what its upstream did.
func TestSelector_ParityAccounting(t *testing.T) {
	// newSeed builds a fresh endpoint per case: a domain copy shares the key
	// slice's backing array, so an outcome recorded on one case's endpoint
	// would park another case's key.
	newSeed := func(t *testing.T) domain.UpstreamEndpoint {
		t.Helper()
		seed := buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
			{id: "uky_a", priority: 1},
		})
		seed.RecordUse(now)
		seed.RecordUse(now)
		// A failure the last sweep recorded, so a success has something to clear.
		seed.RecordUpstreamError("UPSTREAM_ERROR", "the upstream answered 500", now.Add(-time.Minute))
		return seed
	}

	cases := []struct {
		name        string
		keyless     bool
		record      func(*Selector, Selection) error
		wantOutcome bool
		wantCode    string
		wantUses    int
	}{
		{
			name: "a served call clears the recorded error and the use run",
			record: func(s *Selector, sel Selection) error {
				return s.RecordSuccess(context.Background(), sel)
			},
			wantOutcome: true,
			wantUses:    0,
		},
		{
			name:    "a served call on a keyless selection still records parity",
			keyless: true,
			record: func(s *Selector, sel Selection) error {
				return s.RecordSuccess(context.Background(), sel)
			},
			wantOutcome: true,
			wantUses:    0,
		},
		{
			name: "a transient failure is recorded under the wire's upstream code",
			record: func(s *Selector, sel Selection) error {
				return s.RecordFailure(context.Background(), sel,
					"the upstream answered 500", domain.KeyFailureTransient)
			},
			wantOutcome: true,
			wantCode:    CodeUpstreamError,
			wantUses:    2,
		},
		{
			name: "a rate limit is recorded under the wire's rate-limit code",
			record: func(s *Selector, sel Selection) error {
				return s.RecordFailure(context.Background(), sel,
					"the upstream answered 429", domain.KeyFailureRateLimit)
			},
			wantOutcome: true,
			wantCode:    CodeRateLimited,
			wantUses:    2,
		},
		{
			name: "a request-shaped failure records nothing",
			record: func(s *Selector, sel Selection) error {
				return s.RecordFailure(context.Background(), sel,
					"upstream rejected the request", domain.KeyFailureRequest)
			},
			wantUses: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{newSeed(t)}
			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selector.clock = func() time.Time { return now }

			selection := Selection{Endpoint: repo.byProvider["provider-a"][0]}
			if !tc.keyless {
				selection, err = selector.Select(context.Background(), "provider-a")
				if err != nil {
					t.Fatalf("selecting: %v", err)
				}
			}
			if err := tc.record(selector, selection); err != nil {
				t.Fatalf("recording the outcome: %v", err)
			}

			stored, ok := repo.outcome["ep_a"]
			if !tc.wantOutcome {
				if ok {
					t.Fatalf("RecordUpstreamOutcome wrote %+v, want no write at all", stored)
				}
				return
			}
			if !ok {
				t.Fatal("RecordUpstreamOutcome was not called, so the parity state was not persisted")
			}
			if uses := stored.ConsecutiveUseCount(); uses != tc.wantUses {
				t.Fatalf("stored consecutive use count = %d, want %d", uses, tc.wantUses)
			}
			code, message, at := stored.LastError()
			if tc.wantCode == "" {
				if code != "" || message != "" || at != nil {
					t.Fatalf("stored last error = (%q, %q, %v), want it cleared", code, message, at)
				}
				return
			}
			if code != tc.wantCode {
				t.Fatalf("stored error code = %q, want %q", code, tc.wantCode)
			}
			if message != "the upstream answered 500" && message != "the upstream answered 429" {
				t.Fatalf("stored error message = %q, want the upstream's own wording", message)
			}
			if at == nil {
				t.Fatal("stored error instant = nil, want the outcome's instant")
			}
		})
	}
}
