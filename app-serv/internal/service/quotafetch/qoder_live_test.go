//go:build integration && live

// Package quotafetch reads the quota a provider publishes for one of its connections.
//
// @file      internal/service/quotafetch/qoder_live_test.go
// @for       The live Qoder quota read: a personal access token in, the published allocation out.
// @uses      context, os, strings, testing, time.
// @reason    Every other case in this package answers from a stub holding a measured payload, which proves the mapping and nothing about the contract. This one asks the real service, because the three things that broke were all contract assumptions the reference's source could not settle: a Personal Access Token is refused by the quota endpoint and has to be exchanged first, the plan name rides in `userType`, and the reset instant arrives as a year-9999 sentinel that a card would render as the end of time.
//
//	PANNELAI_QODER_PAT='pt-…' \
//	  go test -tags=integration,live ./internal/service/quotafetch/ -run Live
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package quotafetch

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// livePAT reads the personal access token the live cases ask with. It is the only
// credential any test in this repository takes from the environment, and the value
// never lands in a file.
func livePAT(t *testing.T) string {
	t.Helper()
	pat := strings.TrimSpace(os.Getenv("PANNELAI_QODER_PAT"))
	if pat == "" {
		t.Fatal("PANNELAI_QODER_PAT must be set for the live quota read")
	}
	return pat
}

// TestFetchQoderLive reads the allocation the service publishes for the account the
// token belongs to, and asserts what must hold whatever that account has spent.
func TestFetchQoderLive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := Fetch(ctx, "qoder", Credentials{APIKey: livePAT(t)})

	// A refusal to reach the endpoint is the failure this test exists for: the
	// exchange broke, the bearer shape changed, or the path moved. What the
	// account actually holds is a fact about the account.
	for _, refusal := range []string{"credential not available", "Qoder error", "Usage API not implemented"} {
		if strings.Contains(result.Message, refusal) {
			t.Fatalf("the endpoint was never reached: message = %q", result.Message)
		}
	}
	if result.Plan == "" {
		t.Fatalf("plan is empty, want the `userType` the answer names (message %q)", result.Message)
	}
	for _, window := range result.Quotas {
		if window.Label == "" {
			t.Fatalf("a published bucket carries no label: %+v", window)
		}
		if window.Used < 0 || window.Total < 0 {
			t.Fatalf("a published bucket is negative: %+v", window)
		}
		// The sentinel is the measured answer's own shape, so this is the one
		// assertion that can only be made against the live service.
		if !window.ResetAt.IsZero() && window.ResetAt.Year() > 2100 {
			t.Fatalf("the year-9999 reset sentinel reached a window: %+v", window)
		}
	}
	t.Logf("qoder published plan %q over %d bucket(s): %+v (message %q)",
		result.Plan, len(result.Quotas), result.Quotas, result.Message)
}
