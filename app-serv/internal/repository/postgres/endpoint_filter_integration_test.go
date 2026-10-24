//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_filter_integration_test.go
// @for       Every narrowing axis of EndpointFilter against a real server, including the one that must read as "no filter at all".
// @uses      internal/domain, context, testing, time.
// @reason    A zero EndpointFilter is what the chat route reads to find an account, so a clause that answers NULL instead of true for an unset filter empties the whole table and the request fails as NO_PROVIDER_AVAILABLE. That is invisible to a Go-side double and to a fake that ignores the predicate, and it is exactly what a nil slice becomes once pgx encodes it: SQL NULL, not an empty array.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// storeListableAccount stores one no-auth account with an optional tag, so a filter case
// can name the row it expects without depending on ordering.
func storeListableAccount(
	t *testing.T, repo *EndpointRepository, ctx context.Context, id, providerID, tag string, now time.Time,
) {
	t.Helper()

	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, id, domain.UpstreamAuthNone, 1, now)
	if err != nil {
		t.Fatalf("building %s: %v", id, err)
	}
	if tag != "" {
		if _, err := endpoint.AddTag(tag, now); err != nil {
			t.Fatalf("tagging %s: %v", id, err)
		}
	}
	if err := repo.Create(ctx, endpoint); err != nil {
		t.Fatalf("storing %s: %v", id, err)
	}
}

func TestIntegration_EndpointFilterNarrowsOnlyWhatItNames(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)
	storeListableAccount(t, repo, ctx, "ep_free", "list-alpha", "free-tier", now)
	storeListableAccount(t, repo, ctx, "ep_paid", "list-beta", "", now)

	cases := []struct {
		name   string
		filter repository.EndpointFilter
		want   int64
	}{
		{name: "a zero filter reads the whole table", filter: repository.EndpointFilter{}, want: 2},
		{
			name:   "a nil id list narrows nothing",
			filter: repository.EndpointFilter{IDs: nil},
			want:   2,
		},
		{
			name:   "an empty id list narrows nothing either",
			filter: repository.EndpointFilter{IDs: []string{}},
			want:   2,
		},
		{
			name:   "one named id answers one row",
			filter: repository.EndpointFilter{IDs: []string{"ep_free"}},
			want:   1,
		},
		{
			name:   "a tag selects the row wearing it",
			filter: repository.EndpointFilter{Tag: "free-tier"},
			want:   1,
		},
		{
			name:   "a provider narrows to its own",
			filter: repository.EndpointFilter{ProviderID: "list-beta"},
			want:   1,
		},
		{
			name:   "an auth type narrows across providers",
			filter: repository.EndpointFilter{AuthType: string(domain.UpstreamAuthNone)},
			want:   2,
		},
		{
			name:   "a tag and a provider intersect",
			filter: repository.EndpointFilter{Tag: "free-tier", ProviderID: "list-beta"},
			want:   0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, total, err := repo.List(ctx, tc.filter, repository.PageQuery{Page: 1, PerPage: 10})
			if err != nil {
				t.Fatalf("List(%+v) error = %v", tc.filter, err)
			}
			if total != tc.want {
				t.Fatalf("List(%+v) total = %d, want %d", tc.filter, total, tc.want)
			}
		})
	}
}
