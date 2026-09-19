//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/media_override_integration_test.go
// @for       The media override repository against a real server: round trip,
//
//	upsert semantics, ordering, and the migration's CHECK constraint.
//
// @uses      testing, context, time, internal/domain.
// @reason    The upsert is the whole contract — a save must replace the row,
//
//	not add a second one — and only a real server proves the conflict
//	target is the primary key the migration declares. The kind CHECK
//	exists twice (domain and schema) and the second one needs the server.
//
//	Run with:
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// newTestMediaOverrideRepo returns a repository over a clean table.
func newTestMediaOverrideRepo(t *testing.T) *MediaOverrideRepository {
	t.Helper()
	pool := newTestPool(t)
	if _, err := pool.Exec(context.Background(), `TRUNCATE media_provider_settings`); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	return NewMediaOverrideRepository(pool)
}

// mustOverride builds one override or fails the test.
func mustOverride(t *testing.T, providerID string, kind domain.MediaKind, baseURL, model string) domain.MediaOverride {
	t.Helper()
	override, err := domain.NewMediaOverride(providerID, kind, baseURL, model, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatalf("NewMediaOverride(%s/%s) error = %v", providerID, kind, err)
	}
	return override
}

// TestMediaOverrideRepository_RoundTripAndUpsert pins that every column
// survives a write/read cycle and that a second save replaces the row rather
// than adding one.
func TestMediaOverrideRepository_RoundTripAndUpsert(t *testing.T) {
	repo := newTestMediaOverrideRepo(t)
	ctx := context.Background()

	first := mustOverride(t, "openai", domain.MediaKindTTS, "https://tts.example.com/v1", "tts-1")
	if err := repo.Upsert(ctx, first); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	listed, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List() returned %d rows, want 1", len(listed))
	}
	if listed[0].ProviderID() != "openai" || listed[0].Kind() != domain.MediaKindTTS ||
		listed[0].BaseURL() != "https://tts.example.com/v1" || listed[0].DefaultModel() != "tts-1" {
		t.Fatalf("List()[0] = %+v, want the stored override", listed[0])
	}
	if !listed[0].UpdatedAt().Equal(first.UpdatedAt()) {
		t.Fatalf("UpdatedAt() = %v, want %v", listed[0].UpdatedAt(), first.UpdatedAt())
	}

	// A save with an empty base_url is how an operator returns to the
	// registry default, so the empty value must round-trip as empty.
	second := mustOverride(t, "openai", domain.MediaKindTTS, "", "tts-2")
	if err := repo.Upsert(ctx, second); err != nil {
		t.Fatalf("Upsert() again error = %v", err)
	}
	listed, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List() returned %d rows after an upsert, want 1", len(listed))
	}
	if listed[0].BaseURL() != "" || listed[0].DefaultModel() != "tts-2" {
		t.Fatalf("List()[0] = %+v, want the replaced values", listed[0])
	}
}

// TestMediaOverrideRepository_Order pins the read order, so a list route and a
// test see the same sequence.
func TestMediaOverrideRepository_Order(t *testing.T) {
	repo := newTestMediaOverrideRepo(t)
	ctx := context.Background()

	rows := []domain.MediaOverride{
		mustOverride(t, "zulu", domain.MediaKindImage, "https://z.example.com", ""),
		mustOverride(t, "alpha", domain.MediaKindVideo, "https://a.example.com", ""),
		mustOverride(t, "alpha", domain.MediaKindImage, "https://a2.example.com", ""),
	}
	for _, row := range rows {
		if err := repo.Upsert(ctx, row); err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
	}

	listed, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []struct {
		provider string
		kind     domain.MediaKind
	}{
		{"alpha", domain.MediaKindImage},
		{"alpha", domain.MediaKindVideo},
		{"zulu", domain.MediaKindImage},
	}
	if len(listed) != len(want) {
		t.Fatalf("List() returned %d rows, want %d", len(listed), len(want))
	}
	for index, row := range listed {
		if row.ProviderID() != want[index].provider || row.Kind() != want[index].kind {
			t.Fatalf("List()[%d] = %s/%s, want %s/%s", index,
				row.ProviderID(), row.Kind(), want[index].provider, want[index].kind)
		}
	}
}

// TestMediaOverrideRepository_Get pins the read the data plane performs per
// call: it answers for the exact key, and a key with no row is "not stored"
// rather than an error.
func TestMediaOverrideRepository_Get(t *testing.T) {
	repo := newTestMediaOverrideRepo(t)
	ctx := context.Background()

	if err := repo.Upsert(ctx, mustOverride(t, "openai", domain.MediaKindEmbedding,
		"http://127.0.0.1:9000/v1", "text-embedding-3-small")); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	stored, found, err := repo.Get(ctx, "openai", domain.MediaKindEmbedding)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || stored.BaseURL() != "http://127.0.0.1:9000/v1" {
		t.Fatalf("Get() = %+v (found %v), want the stored row", stored, found)
	}

	for _, tc := range []struct {
		name       string
		providerID string
		kind       domain.MediaKind
	}{
		{name: "another kind", providerID: "openai", kind: domain.MediaKindTTS},
		{name: "another provider", providerID: "elevenlabs", kind: domain.MediaKindEmbedding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, found, err := repo.Get(ctx, tc.providerID, tc.kind)
			if err != nil {
				t.Fatalf("Get() error = %v, want no error for a missing row", err)
			}
			if found {
				t.Fatalf("Get(%s/%s) found a row, want none", tc.providerID, tc.kind)
			}
		})
	}
}

// TestMediaOverrideRepository_KindConstraint pins the migration's CHECK: a kind
// outside the closed set is refused at rest even if a caller casts past the
// domain parser.
func TestMediaOverrideRepository_KindConstraint(t *testing.T) {
	repo := newTestMediaOverrideRepo(t)
	ctx := context.Background()

	cases := []struct {
		name string
		kind string
	}{
		{name: "the registry's own spelling", kind: "webSearch"},
		{name: "the panel's label", kind: "web"},
		{name: "a kind outside the set", kind: "music"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := domain.RehydrateMediaOverride("openai", domain.MediaKind(tc.kind),
				"https://api.example.com", "", time.Now().UTC())
			if err := repo.Upsert(ctx, raw); err == nil {
				t.Fatalf("Upsert() accepted kind %q, want the constraint to refuse it", tc.kind)
			}
		})
	}
}
