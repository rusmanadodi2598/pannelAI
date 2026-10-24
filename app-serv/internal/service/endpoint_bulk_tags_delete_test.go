// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_bulk_tags_delete_test.go
// @for       The bulk retag and bulk delete over accounts: what lands on every row of a resolved set, and what a bad row or a vanished row does to the rest.
// @uses      context, testing, internal/domain.
// @reason    Retagging and erasing are the two bulk actions whose per-row outcomes a client has to render, and each has a failure that only shows up across a set: one malformed label must refuse that row and not the neighbors, while one vanished account must abort the whole erase. Split from the resolution tests so each stays inside the AGENTS.md §1.1 line budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestBulkTagsAddsRemovesAndClearsAcrossTheSet(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("deepseek"), newTestSealer(t), nil)
	seedBulkAccount(t, store, "ep_a", "deepseek", "free-tier")
	seedBulkAccount(t, store, "ep_b", "deepseek")

	if _, err := svc.BulkTags(context.Background(), BulkSelector{All: true},
		[]string{"Event_Week"}, nil, false, 2); err != nil {
		t.Fatalf("BulkTags() add error = %v", err)
	}
	for _, id := range []string{"ep_a", "ep_b"} {
		tags := mustStored(t, store, id).Tags()
		if len(tags) == 0 || tags[len(tags)-1] != "event_week" {
			t.Fatalf("%s tags = %v, want the normalized label on every row", id, tags)
		}
	}

	if _, err := svc.BulkTags(context.Background(), BulkSelector{IDs: []string{"ep_a"}},
		nil, []string{"free-tier"}, false, 1); err != nil {
		t.Fatalf("BulkTags() remove error = %v", err)
	}
	if holdsTag(mustStored(t, store, "ep_a").Tags(), "free-tier") {
		t.Fatal("the named tag survived its removal")
	}

	if _, err := svc.BulkTags(context.Background(), BulkSelector{All: true},
		nil, nil, true, 2); err != nil {
		t.Fatalf("BulkTags() clear error = %v", err)
	}
	for _, id := range []string{"ep_a", "ep_b"} {
		if got := mustStored(t, store, id).Tags(); len(got) != 0 {
			t.Fatalf("%s tags = %v, want the set cleared", id, got)
		}
	}
}

func TestBulkTagsRefusesOneBadRowWithoutLosingTheOthers(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("deepseek"), newTestSealer(t), nil)
	seedBulkAccount(t, store, "ep_a", "deepseek")
	seedBulkAccount(t, store, "ep_b", "deepseek")

	// One malformed label cannot be stored, and the other row is not its to lose: the
	// refusal is reported on the row the operator named wrongly.
	outcome, err := svc.BulkTags(context.Background(), BulkSelector{IDs: []string{"ep_a", "ep_b"}},
		[]string{"Bad Space"}, nil, false, 2)
	if err != nil {
		t.Fatalf("BulkTags() error = %v", err)
	}
	refused := 0
	for _, row := range outcome.Results {
		if row.Error != nil {
			refused++
			if domain.AsAppError(row.Error).Code != "VALIDATION_ERROR" {
				t.Fatalf("row %d code = %q, want VALIDATION_ERROR", row.Index, domain.AsAppError(row.Error).Code)
			}
		}
	}
	if refused != 2 {
		t.Fatalf("refused rows = %d, want the one bad label refused on both", refused)
	}
	if len(mustStored(t, store, "ep_a").Tags()) != 0 {
		t.Fatal("a refused tag landed anyway")
	}
}

func TestBulkTagsNeedsAnIntent(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	seedBulkAccount(t, store, "ep_a", "deepseek")
	_, err := svc.BulkTags(context.Background(), BulkSelector{All: true}, nil, nil, false, 1)
	if domain.AsAppError(err).Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %q, want a refusal that names the missing intent", domain.AsAppError(err).Code)
	}
}

func TestBulkDeleteRemovesTheSetAndItsKeys(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	endpoint := seedEndpoint(t, svc, "deepseek", "acct", domain.UpstreamAuthAPIKey,
		KeyInput{Label: "primary", Value: "sk-one"})
	stored := mustStored(t, store, endpoint.ID())
	if len(stored.Keys()) == 0 {
		t.Fatal("the fixture has no key for the cascade to take")
	}

	outcome, err := svc.BulkDelete(context.Background(), BulkSelector{IDs: []string{endpoint.ID()}}, 1)
	if err != nil {
		t.Fatalf("BulkDelete() error = %v", err)
	}
	if outcome.Matched != 1 || outcome.Changed != 1 {
		t.Fatalf("outcome = %+v, want the one account removed", outcome)
	}
	if _, err := store.GetByID(context.Background(), endpoint.ID()); err != domain.ErrEndpointNotFound {
		t.Fatalf("reload error = %v, want the account gone", err)
	}
	if left := len(store.keysByEndpoint[endpoint.ID()]); left != 0 {
		t.Fatalf("%d key row(s) outlived the delete that erased their account", left)
	}
}

// TestBulkDeleteRefusesAStaleSelection proves the guard is not decoration: a row that
// vanished between the operator's render and their click stops the whole action rather
// than deleting whatever remains of the set.
func TestBulkDeleteRefusesAStaleSelection(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	first := seedBulkAccount(t, store, "ep_a", "deepseek")
	seedBulkAccount(t, store, "ep_b", "deepseek")
	if err := store.Delete(context.Background(), first.ID()); err != nil {
		t.Fatalf("removing one row: %v", err)
	}

	_, err := svc.BulkDelete(context.Background(), BulkSelector{IDs: []string{"ep_a", "ep_b"}}, 2)
	if err == nil {
		t.Fatal("a selection naming a vanished row was accepted")
	}
	if _, reloadErr := store.GetByID(context.Background(), "ep_b"); reloadErr != nil {
		t.Fatalf("the unvanished row went with the refused delete: %v", reloadErr)
	}
}

func TestBulkSelectorResolvesAuthTypeNarrowing(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("deepseek"), newTestSealer(t), nil)
	seedBulkAccount(t, store, "ep_plain", "deepseek")
	keyed, err := domain.NewUpstreamEndpoint("ep_keyed", "deepseek", "keyed", domain.UpstreamAuthAPIKey, 1, testNow)
	if err != nil {
		t.Fatalf("building the keyed account: %v", err)
	}
	if _, err := keyed.AddKey("primary", "sealed", "sk-…cret", 0, testNow); err != nil {
		t.Fatalf("seeding its key: %v", err)
	}
	if err := store.Create(context.Background(), keyed); err != nil {
		t.Fatalf("storing the keyed account: %v", err)
	}

	outcome, err := svc.BulkStatus(context.Background(),
		BulkSelector{All: true, AuthType: domain.UpstreamAuthAPIKey}, "disabled", 0, 1)
	if err != nil {
		t.Fatalf("BulkStatus() error = %v", err)
	}
	if outcome.Matched != 1 {
		t.Fatalf("matched = %d, want only the account that authenticates with a key", outcome.Matched)
	}
	if mustStored(t, store, "ep_plain").Status() != domain.UpstreamEndpointActive {
		t.Fatal("an unkeyed account was disabled by a key-scoped selector")
	}
}
