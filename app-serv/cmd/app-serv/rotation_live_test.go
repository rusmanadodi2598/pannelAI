//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/rotation_live_test.go
// @for       The rotation proof through the real stores: a burst of concurrent legs over one account's keys, against real Redis and real PostgreSQL.
// @uses      internal/dataplane, internal/domain, internal/repository/postgres, internal/repository/redis, internal/service, context, strings, sync, testing, time.
// @reason    SPEC-API-001 §7.5 claims a burst spreads across an account's credentials, and every test that proved it so far used an in-memory cursor standing in for Redis and a stub endpoint repository standing in for PostgreSQL. The claim is about two processes choosing at the same instant, so it is only evidence when the arbiter and the rows are the real ones; this is that evidence, wired from the same constructors the composition root uses.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-14
package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

const (
	// A provider of our own: the shared live harness seeds one no_auth endpoint under "live", and a
	// leg that lands on that account answers with no key at all, which is correct and not what this
	// burst is measuring. One account here means every leg competes for the same key set.
	rotationProvider = "rr-live"
	// rotationLiveProvider is the one id the shared harness can actually route: its
	// registry entry and stub upstream are built for "live".
	rotationLiveProvider = "live"
	rotationEndpointID   = "ep-rotation"
	rotationKeyCount     = 5
)

// TestLiveRotation_SpreadsAConcurrentBurstAcrossOneAccount'sKeys walks the path a
// real request takes at the moment the old defect bit: N legs asking one account for
// a credential at the same time.
func TestLiveRotation_SpreadsAConcurrentBurst(t *testing.T) {
	stack := newLiveStack(t, newLiveUpstream(t))
	ctx := context.Background()
	now := time.Now().UTC()

	sealer, err := domain.NewSealer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("building the sealer: %v", err)
	}
	endpoints := postgres.NewEndpointRepository(stack.pool)

	endpoint, err := domain.NewUpstreamEndpoint(
		rotationEndpointID, rotationProvider, "rotation account", domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("building the endpoint: %v", err)
	}
	for i := range rotationKeyCount {
		label := "key-" + string(rune('a'+i))
		sealed, err := sealer.Seal("sk-secret-" + label)
		if err != nil {
			t.Fatalf("sealing %s: %v", label, err)
		}
		key, err := domain.NewUpstreamKey(label, rotationEndpointID, "", sealed, "sk-"+label, 1, now)
		if err != nil {
			t.Fatalf("building %s: %v", label, err)
		}
		endpoint.AttachKey(key)
	}
	if err := endpoints.Create(ctx, endpoint); err != nil {
		t.Fatalf("storing the account: %v", err)
	}

	// The policy is written through the real settings service rather than stubbed:
	// which mode a provider resolves to is part of what a live burst exercises.
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

	selector, err := dataplane.NewSelector(dataplane.SelectorDeps{
		Endpoints: endpoints, Opener: sealer,
		Cursor: redisrepo.NewCursorStore(stack.redis), KeyCursor: redisrepo.NewKeyCursorStore(stack.redis),
		Strategies: settings,
	})
	if err != nil {
		t.Fatalf("building the selector: %v", err)
	}

	// Every leg is a separate request that has spent nothing yet, which is the
	// shape that used to collapse onto one credential: each read the same
	// last_used_at and each picked the same "least recently used" key.
	chosen := make([]string, rotationKeyCount)
	var group sync.WaitGroup
	for i := range rotationKeyCount {
		group.Add(1)
		go func(slot int) {
			defer group.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("leg %d panicked: %v", slot, r)
				}
			}()
			plan, err := selector.BeginPlan(ctx, rotationProvider)
			if err != nil {
				t.Errorf("leg %d BeginPlan() error = %v", slot, err)
				return
			}
			selection, ok, err := plan.Next(ctx, nil)
			if err != nil {
				t.Errorf("leg %d Next() error = %v", slot, err)
				return
			}
			if !ok {
				t.Errorf("leg %d found no credential", slot)
				return
			}
			chosen[slot] = selection.Key.ID()
		}(i)
	}
	group.Wait()

	distinct := map[string]bool{}
	for _, id := range chosen {
		if id == "" {
			t.Fatalf("a leg chose no key; the burst was %v", chosen)
		}
		distinct[id] = true
	}
	if len(distinct) != rotationKeyCount {
		t.Fatalf("the burst chose %d distinct keys of %d over %d concurrent legs (%v); want one key each",
			len(distinct), rotationKeyCount, rotationKeyCount, chosen)
	}

	// The stamps must be in PostgreSQL, not only in memory: they are what orders the
	// next selection when Redis cannot answer, and what the panel shows as chosen.
	stored, err := endpoints.GetByID(ctx, rotationEndpointID)
	if err != nil {
		t.Fatalf("reading the account back: %v", err)
	}
	unstamped := 0
	for _, key := range stored.Keys() {
		if key.LastSelectedAt() == nil {
			unstamped++
		}
	}
	if unstamped != 0 {
		t.Fatalf("%d of %d keys carry no selection stamp, want every chosen key stamped",
			unstamped, rotationKeyCount)
	}
	if stored.LastSelectedAt() == nil {
		t.Fatal("the account carries no selection stamp, want the key and the account advanced together")
	}
}
