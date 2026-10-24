//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/rotation_live_accounts_test.go
// @for       The multi-account half of the rotation proof: real chat requests through the live router, with the usage rows as the evidence.
// @uses      context, net/http, testing, time, internal/domain, internal/repository/postgres, internal/service.
// @reason    "Every provider supports multiple keys or multiple accounts" is two claims, and only one of them is visible in the same place. usage_records carries endpoint_id and the caller's gateway_key_id but never the upstream key id, so a multi-key spread is only observable in upstream_keys.last_selected_at while a multi-account spread is observable here. This is the half the sibling test cannot show.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-14
package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TestLiveRotation_SpreadsRequestsAcrossAccounts is the other half of the requirement,
// through the whole HTTP path rather than the selector alone: several accounts under one
// provider, real requests, the usage rows as evidence. usage_records carries endpoint_id
// and the caller's gateway_key_id but never the upstream key id, so a multi-key spread is
// only visible in last_selected_at, which the sibling test reads.
func TestLiveRotation_SpreadsRequestsAcrossAccounts(t *testing.T) {
	upstream := newLiveUpstream(t)
	stack := newLiveStack(t, upstream)
	ctx := context.Background()
	now := time.Now().UTC()

	if _, err := stack.pool.Exec(ctx, "DELETE FROM upstream_endpoints WHERE provider_id = $1", rotationLiveProvider); err != nil {
		t.Fatalf("clearing the provider's accounts: %v", err)
	}
	endpoints := postgres.NewEndpointRepository(stack.pool)
	const accountCount = 3
	for i := range accountCount {
		id := "ep-rr-" + string(rune('a'+i))
		account, err := domain.NewUpstreamEndpoint(id, rotationLiveProvider, "account "+id, domain.UpstreamAuthNone, i+1, now)
		if err != nil {
			t.Fatalf("building %s: %v", id, err)
		}
		if err := endpoints.Create(ctx, account); err != nil {
			t.Fatalf("storing %s: %v", id, err)
		}
	}

	settings, err := service.NewSettingsService(service.SettingsServiceDeps{
		Repo: postgres.NewSettingsRepository(stack.pool),
	})
	if err != nil {
		t.Fatalf("building the settings service: %v", err)
	}
	roundRobin := domain.RotationRoundRobin
	sticky := 1
	if _, err := settings.Update(ctx, domain.SettingsPatch{Routing: &domain.RoutingSettingsPatch{
		FallbackStrategy: &roundRobin, StickyLimit: &sticky,
	}}); err != nil {
		t.Fatalf("setting round-robin: %v", err)
	}

	// Six requests over three accounts with a sticky limit of 1: the cursor moves
	// every call, so each account must serve exactly two and none may be skipped.
	body := `{"model":"live/live-model","messages":[{"role":"user","content":"ping"}]}`
	for i := range accountCount * 2 {
		status, answered, _ := stack.post(t, body, stack.key, "")
		if status != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200; body = %s", i+1, status, answered)
		}
	}

	rows, err := stack.pool.Query(ctx, "SELECT endpoint_id, count(*) FROM usage_records GROUP BY endpoint_id")
	if err != nil {
		t.Fatalf("reading the usage rows: %v", err)
	}
	defer rows.Close()
	spread := map[string]int{}
	for rows.Next() {
		var id string
		var seen int
		if err := rows.Scan(&id, &seen); err != nil {
			t.Fatalf("scanning a usage row: %v", err)
		}
		spread[id] = seen
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the usage rows: %v", err)
	}
	if len(spread) != accountCount {
		t.Fatalf("the requests reached %d distinct accounts (%v), want all %d",
			len(spread), spread, accountCount)
	}
	for id, seen := range spread {
		if seen != 2 {
			t.Fatalf("account %s served %d of %d requests, want 2 each: %v", id, seen, accountCount*2, spread)
		}
	}
}
