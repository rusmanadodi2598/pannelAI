//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_key_rotation_integration_test.go
// @for       The store half of rotation precision: per-class runs counted inside the database, and a selection stamp that survives an outcome write.
// @uses      context, testing, time, internal/domain
// @reason    A run counted from the aggregate would land one increment where two selectors recorded two failures, and an outcome write that carried the loaded selection stamp backwards would undo a choice made while the call was still running. Neither failure is visible to a unit test against a fake, because both are about what two writers do to one row.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-14
package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestIntegration_PerClassRunsCountSeparatelyInTheStore pins that a rate limit and a
// credential failure each advance their own column, and that the shared total still
// sums them for display.
func TestIntegration_PerClassRunsCountSeparatelyInTheStore(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	seedHealthKey(t, repo, 0, now)

	for range 3 {
		loaded, err := repo.GetByID(ctx, healthEndpointID)
		if err != nil {
			t.Fatalf("reading back: %v", err)
		}
		updated, err := loaded.RecordKeyFailure(healthKeyID, "throttled",
			domain.KeyFailureRateLimit, domain.DefaultKeyHealthPolicy(), now)
		if err != nil {
			t.Fatalf("recording a throttle: %v", err)
		}
		if err := repo.RecordKeyHealth(ctx, updated); err != nil {
			t.Fatalf("RecordKeyHealth() error = %v", err)
		}
	}

	stored := readHealthKey(t, repo, ctx).Key(healthKeyID)
	if got := stored.ConsecutiveRateLimit(); got != 3 {
		t.Fatalf("rate-limit run = %d, want 3", got)
	}
	if got := stored.ConsecutiveAuth(); got != 0 {
		t.Fatalf("auth run = %d, want 0: a throttle must not advance the credential ladder", got)
	}
	if got := stored.ConsecutiveErrors(); got != 3 {
		t.Fatalf("total errors = %d, want 3", got)
	}
	if got := stored.LastErrorClass(); got != "ratelimit" {
		t.Fatalf("last error class = %q, want ratelimit", got)
	}

	// The refusal that follows steps on a run of one. Read the row back first, the
	// way a selector does, so the store is what decided the count.
	loaded, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	refused, err := loaded.RecordKeyFailure(healthKeyID, "refused",
		domain.KeyFailureAuth, domain.DefaultKeyHealthPolicy(), now)
	if err != nil {
		t.Fatalf("recording a refusal: %v", err)
	}
	if err := repo.RecordKeyHealth(ctx, refused); err != nil {
		t.Fatalf("RecordKeyHealth() error = %v", err)
	}
	if got := readHealthKey(t, repo, ctx).Key(healthKeyID).ConsecutiveAuth(); got != 1 {
		t.Fatalf("auth run = %d, want 1: the three throttles were not the credential's", got)
	}
}

// TestIntegration_SuccessClearsEveryRun pins the reset across all three columns plus
// the class name, in one served call.
func TestIntegration_SuccessClearsEveryRun(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	seedHealthKey(t, repo, 0, now)
	for _, class := range []domain.KeyFailureClass{
		domain.KeyFailureAuth, domain.KeyFailureRateLimit, domain.KeyFailureBilling,
	} {
		loaded, err := repo.GetByID(ctx, healthEndpointID)
		if err != nil {
			t.Fatalf("reading back: %v", err)
		}
		failed, err := loaded.RecordKeyFailure(healthKeyID, "no", class, domain.DefaultKeyHealthPolicy(), now)
		if err != nil {
			t.Fatalf("recording %v: %v", class, err)
		}
		if err := repo.RecordKeyHealth(ctx, failed); err != nil {
			t.Fatalf("RecordKeyHealth() error = %v", err)
		}
	}

	loaded, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	served, err := loaded.RecordKeySuccess(healthKeyID, now)
	if err != nil {
		t.Fatalf("RecordKeySuccess() error = %v", err)
	}
	if err := repo.RecordKeyHealth(ctx, served); err != nil {
		t.Fatalf("RecordKeyHealth() error = %v", err)
	}

	stored := readHealthKey(t, repo, ctx).Key(healthKeyID)
	for name, got := range map[string]int{
		"total":     stored.ConsecutiveErrors(),
		"auth":      stored.ConsecutiveAuth(),
		"ratelimit": stored.ConsecutiveRateLimit(),
		"billing":   stored.ConsecutiveBilling(),
	} {
		if got != 0 {
			t.Fatalf("%s run = %d, want 0 after a served call", name, got)
		}
	}
	if stored.LastErrorClass() != "" {
		t.Fatalf("last error class = %q, want it cleared with the error", stored.LastErrorClass())
	}
}

// TestIntegration_ConcurrentFailuresCountTwice pins the reason the counters live in
// SQL: two writers that loaded the same starting value must still land two strikes.
func TestIntegration_ConcurrentFailuresCountTwice(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	seedHealthKey(t, repo, 0, now)
	loaded, err := repo.GetByID(ctx, healthEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	// Both writers read the same row, which is the interleaving under test.
	first, err := loaded.RecordKeyFailure(healthKeyID, "throttled",
		domain.KeyFailureRateLimit, domain.DefaultKeyHealthPolicy(), now)
	if err != nil {
		t.Fatalf("recording the first throttle: %v", err)
	}
	second, err := loaded.RecordKeyFailure(healthKeyID, "throttled",
		domain.KeyFailureRateLimit, domain.DefaultKeyHealthPolicy(), now)
	if err != nil {
		t.Fatalf("recording the second throttle: %v", err)
	}

	var group sync.WaitGroup
	writes := []domain.UpstreamKey{first, second}
	for _, write := range writes {
		group.Add(1)
		go func(key domain.UpstreamKey) {
			defer group.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("health write panicked: %v", r)
				}
			}()
			if err := repo.RecordKeyHealth(ctx, key); err != nil {
				t.Errorf("RecordKeyHealth() error = %v", err)
			}
		}(write)
	}
	group.Wait()

	stored := readHealthKey(t, repo, ctx).Key(healthKeyID)
	if got := stored.ConsecutiveRateLimit(); got != 2 {
		t.Fatalf("rate-limit run = %d, want 2: one row read by two writers is two strikes", got)
	}
	if stored.RateLimitedUntil() == nil {
		t.Fatal("the key is not parked, want the merged deadline")
	}
}
