//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_operator_marks_integration_test.go
// @for       The round trip of an operator's marks through the row codec: tags and the disable window, on the account and on its key, plus the containment the bulk selector runs.
// @uses      internal/domain, context, testing, time.
// @reason    A fake map stores whatever Go put in it and answers a struct field, so it cannot show that a jsonb array survived the codec, that a deadline kept its instant through timestamptz, or that `@>` reaches the GIN index the selector depends on. Those three are the parts of this feature a real server can break, and the tag list a bulk action resolves is only correct if the stored row reads back with the same labels.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-16
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestIntegration_OperatorMarksRoundTripThroughTheRowCodec(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)
	deadline := now.Add(24 * time.Hour)

	endpoint, err := domain.NewUpstreamEndpoint("ep_marks", "provider-marks", "primary", domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("building the endpoint: %v", err)
	}
	key, err := domain.NewUpstreamKey("primary", "ep_marks", "k_marks", "sealed", "sk-…cret", 1, now)
	if err != nil {
		t.Fatalf("building the key: %v", err)
	}
	if _, err := endpoint.AddTag("Event_Week", now); err != nil {
		t.Fatalf("tagging the endpoint: %v", err)
	}
	if _, err := endpoint.AddTag("free-tier", now); err != nil {
		t.Fatalf("tagging the endpoint again: %v", err)
	}
	endpoint.Disable(deadline, now)
	key.Disable(deadline, now)
	endpoint.AttachKey(key)
	if err := repo.Create(ctx, endpoint); err != nil {
		t.Fatalf("storing the endpoint: %v", err)
	}

	stored, err := repo.GetByID(ctx, "ep_marks")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if got := stored.Tags(); len(got) != 2 || got[0] != "event_week" || got[1] != "free-tier" {
		t.Fatalf("endpoint tags = %v, want the two labels in the order stored", got)
	}
	if until := stored.DisabledUntil(); until == nil || !until.Equal(deadline) {
		t.Fatalf("endpoint window = %v, want %v", until, deadline)
	}
	storedKeys := stored.Keys()
	if len(storedKeys) != 1 {
		t.Fatalf("stored keys = %d, want the one key attached", len(storedKeys))
	}
	if until := storedKeys[0].DisabledUntil(); until == nil || !until.Equal(deadline) {
		t.Fatalf("key window = %v, want %v", until, deadline)
	}

	// The key's own write path carries the marks too, because that is the one a bulk
	// action on keys uses: a re-read must show what UpdateKey stored, not what create did.
	if _, err := storedKeys[0].AddTag("key-side", now); err != nil {
		t.Fatalf("tagging the key: %v", err)
	}
	if err := repo.UpdateKey(ctx, storedKeys[0]); err != nil {
		t.Fatalf("UpdateKey(): %v", err)
	}
	reloaded, err := repo.GetByID(ctx, "ep_marks")
	if err != nil {
		t.Fatalf("reloading after the key write: %v", err)
	}
	if got := reloaded.Keys()[0].Tags(); len(got) != 1 || got[0] != "key-side" {
		t.Fatalf("key tags = %v, want the label UpdateKey stored", got)
	}
}

// TestIntegration_TagContainmentFindsTheRowTheSelectorNames pins the predicate a bulk
// action resolves against: one label answers the rows that hold it and no others, which
// is the question the GIN index in migration 000022 exists to answer.
func TestIntegration_TagContainmentFindsTheRowTheSelectorNames(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)

	for _, id := range []string{"ep_free", "ep_paid", "ep_second_free"} {
		endpoint, err := domain.NewUpstreamEndpoint(id, "provider-tags", id, domain.UpstreamAuthNone, 1, now)
		if err != nil {
			t.Fatalf("building %s: %v", id, err)
		}
		tag := "free-tier"
		if id == "ep_paid" {
			tag = "paid-tier"
		}
		if _, err := endpoint.AddTag(tag, now); err != nil {
			t.Fatalf("tagging %s: %v", id, err)
		}
		if err := repo.Create(ctx, endpoint); err != nil {
			t.Fatalf("storing %s: %v", id, err)
		}
	}

	const matching = `SELECT count(*) FROM upstream_endpoints WHERE tags @> '["free-tier"]'::jsonb`
	var count int
	if err := repo.pool.QueryRow(ctx, matching).Scan(&count); err != nil {
		t.Fatalf("running the containment query: %v", err)
	}
	if count != 2 {
		t.Fatalf("rows holding the tag = %d, want the two free-tier accounts", count)
	}

	const byTag = `
SELECT count(*) FROM upstream_endpoints
 WHERE tags @> $1::jsonb AND provider_id = $2`
	var scoped int
	if err := repo.pool.QueryRow(ctx, byTag, `["paid-tier"]`, "provider-tags").Scan(&scoped); err != nil {
		t.Fatalf("running the scoped containment: %v", err)
	}
	if scoped != 1 {
		t.Fatalf("provider-scoped rows = %d, want the one paid account", scoped)
	}
}
