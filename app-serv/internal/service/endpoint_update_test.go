// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_update_test.go
// @for       The PATCH and DELETE paths of the endpoint service.
// @uses      context, testing, internal/domain.
// @reason    Renumbering siblings and refusing to delete the last usable account
//
//	are rules about the whole collection, so they are pinned apart from
//	the create path that only ever appends one row.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04

package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestEndpointService_Update pins the PATCH contract: an omitted field keeps its
// value, an invalid one is refused, and a priority change renumbers the provider's
// siblings so no two endpoints share a slot (§7.5).
func TestEndpointService_Update(t *testing.T) {
	t.Run("partial patches leave omitted fields alone", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		ctx := context.Background()
		endpoint, err := svc.Create(ctx, CreateInput{ProviderID: "deepseek", Label: "keep", AuthType: domain.UpstreamAuthOAuth, Priority: 3})
		if err != nil {
			t.Fatal(err)
		}

		label := "renamed"
		updated, err := svc.Update(ctx, endpoint.ID(), UpdatePatch{Label: &label})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.Label() != "renamed" {
			t.Fatalf("label = %q, want renamed", updated.Label())
		}
		if updated.Priority() != 3 {
			t.Fatalf("priority = %d, want the omitted field untouched at 3", updated.Priority())
		}

		status := "disabled"
		updated, err = svc.Update(ctx, endpoint.ID(), UpdatePatch{Status: &status})
		if err != nil {
			t.Fatalf("Update(status) error = %v", err)
		}
		if updated.Status() != domain.UpstreamEndpointDisabled {
			t.Fatalf("status = %q, want disabled", updated.Status())
		}
		if updated.Label() != "renamed" {
			t.Fatalf("label = %q, want the omitted field untouched", updated.Label())
		}
	})

	t.Run("invalid patches are refused", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		ctx := context.Background()
		endpoint, err := svc.Create(ctx, CreateInput{ProviderID: "deepseek", Label: "bad", AuthType: domain.UpstreamAuthOAuth})
		if err != nil {
			t.Fatal(err)
		}

		blank, zero, retired := "  ", 0, "retired"
		cases := []struct {
			name     string
			patch    UpdatePatch
			wantCode string
		}{
			{"blank label", UpdatePatch{Label: &blank}, "VALIDATION_ERROR"},
			{"zero priority", UpdatePatch{Priority: &zero}, "VALIDATION_ERROR"},
			{"unknown status", UpdatePatch{Status: &retired}, "VALIDATION_ERROR"},
			{"empty patch is a no-op", UpdatePatch{}, ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := svc.Update(ctx, endpoint.ID(), tc.patch)
				if tc.wantCode == "" {
					if err != nil {
						t.Fatalf("Update() error = %v", err)
					}
					return
				}
				mustAppError(t, err, tc.wantCode)
			})
		}
	})

	t.Run("unknown id is not found", func(t *testing.T) {
		svc, _ := newEndpointSvc(t)
		label := "x"
		_, err := svc.Update(context.Background(), "ep_missing", UpdatePatch{Label: &label})
		mustAppError(t, err, "NOT_FOUND")
	})
}

// TestEndpointService_UpdateRenumbersSiblings pins the rule §7.5 states: moving one
// endpoint's priority reorders the provider's set transactionally, so the moved
// endpoint lands where the operator put it and no two share a slot.
func TestEndpointService_UpdateRenumbersSiblings(t *testing.T) {
	cases := []struct {
		name        string
		moveLabel   string
		newPriority int
		wantOrder   []string
	}{
		{name: "move the third to the front", moveLabel: "c", newPriority: 1, wantOrder: []string{"c", "a", "b"}},
		{name: "move the first to the middle", moveLabel: "a", newPriority: 2, wantOrder: []string{"b", "a", "c"}},
		{name: "move the first to the end", moveLabel: "a", newPriority: 3, wantOrder: []string{"b", "c", "a"}},
		{name: "a priority past the end clamps to last", moveLabel: "a", newPriority: 99, wantOrder: []string{"b", "c", "a"}},
		{name: "an unchanged priority keeps the order", moveLabel: "b", newPriority: 2, wantOrder: []string{"a", "b", "c"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			ctx := context.Background()

			ids := map[string]string{}
			for i, label := range []string{"a", "b", "c"} {
				endpoint, err := svc.Create(ctx, CreateInput{
					ProviderID: "deepseek", Label: label, AuthType: domain.UpstreamAuthOAuth, Priority: i + 1,
				})
				if err != nil {
					t.Fatal(err)
				}
				ids[label] = endpoint.ID()
			}

			if _, err := svc.Update(ctx, ids[tc.moveLabel], UpdatePatch{Priority: &tc.newPriority}); err != nil {
				t.Fatalf("Update() error = %v", err)
			}

			ordered, _, err := svc.List(ctx, "deepseek", "", 1, 10)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			got := make([]string, 0, len(ordered))
			for _, endpoint := range ordered {
				got = append(got, endpoint.Label())
			}
			// The comparison is over the set of priorities, not the stored order,
			// because the stub's List is map-backed: what the rule promises is that
			// each endpoint holds a distinct slot and the moved one holds its new
			// position.
			for i, label := range tc.wantOrder {
				want := i + 1
				if priorityOf(t, ordered, ids[label]) != want {
					t.Fatalf("label %q has priority %d, want %d (order: %v)",
						label, priorityOf(t, ordered, ids[label]), want, got)
				}
			}
		})
	}
}

// TestEndpointService_Delete pins delete and its not-found path.
func TestEndpointService_Delete(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	ctx := context.Background()
	endpoint, err := svc.Create(ctx, CreateInput{ProviderID: "deepseek", Label: "gone", AuthType: domain.UpstreamAuthOAuth})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		id       string
		wantCode string
	}{
		{"an existing endpoint is deleted", endpoint.ID(), ""},
		{"a second delete is not found", endpoint.ID(), "NOT_FOUND"},
		{"an unknown id is not found", "ep_absent", "NOT_FOUND"},
		{"a blank id is a validation failure", "  ", "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.Delete(ctx, tc.id)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				return
			}
			mustAppError(t, err, tc.wantCode)
		})
	}
}

// priorityOf reports the priority an endpoint holds inside a listed page.
func priorityOf(t *testing.T, page []domain.UpstreamEndpoint, id string) int {
	t.Helper()
	for _, endpoint := range page {
		if endpoint.ID() == id {
			return endpoint.Priority()
		}
	}
	t.Fatalf("endpoint %q is absent from the page", id)
	return 0
}
