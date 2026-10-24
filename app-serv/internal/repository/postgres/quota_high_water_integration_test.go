//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_high_water_integration_test.go
// @for       The computed quota high-water mark and the reset that replaces it.
// @uses      context, testing, time, internal/domain.
// @reason    An out-of-order flush from a slower replica must not lower a newer computed total, while a rolled-over window must still start from its new value.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-22
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestQuotaRepository_ComputedHighWaterMark(t *testing.T) {
	repo := newTestQuotaRepo(t)
	ctx := context.Background()
	reset := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	upsert := func(used int64, resets *time.Time) {
		t.Helper()
		window := domain.RehydrateQuotaWindow("ep_hw", "", domain.QuotaWindowDaily, used,
			nil, resets, domain.QuotaSourceComputed, time.Time{})
		if err := repo.UpsertWindows(ctx, []domain.QuotaWindow{window}); err != nil {
			t.Fatalf("UpsertWindows(%d) error = %v", used, err)
		}
	}

	upsert(150, &reset)
	upsert(100, &reset)
	windows, err := repo.ListWindows(ctx, "ep_hw")
	if err != nil {
		t.Fatalf("ListWindows() error = %v", err)
	}
	if len(windows) != 1 || windows[0].Used() != 150 {
		t.Fatalf("used = %v, want the newer 150 kept", windows)
	}

	next := reset.Add(time.Hour)
	upsert(25, &next)
	windows, err = repo.ListWindows(ctx, "ep_hw")
	if err != nil {
		t.Fatalf("ListWindows() error = %v", err)
	}
	if windows[0].Used() != 25 {
		t.Fatalf("used after reset = %d, want 25", windows[0].Used())
	}
}
