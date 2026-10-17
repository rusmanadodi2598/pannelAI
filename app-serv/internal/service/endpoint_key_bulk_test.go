// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_key_bulk_test.go
// @for       The bulk actions over credentials: the set-scoped floor that keeps an account usable, and the window sized by each key's own provider.
// @uses      context, testing, time, internal/domain.
// @reason    "This endpoint must keep at least one active key" is a rule about a collection, and a bulk action is where testing it row by row would lie: two keys of one account each pass the check on their own and together leave the account with nothing. Every test here therefore selects both keys of one account and asserts the refusal lands on the row that would have emptied it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// seedKeyedAccount stores one api_key account with n credentials and answers them in the
// order rotation would read them.
func seedKeyedAccount(t *testing.T, svc *EndpointService, providerID, label string, keys int) domain.UpstreamEndpoint {
	t.Helper()
	inputs := make([]KeyInput, 0, keys)
	for i := range keys {
		inputs = append(inputs, KeyInput{Label: "key-" + string(rune('a'+i)), Value: "sk-secret-" + string(rune('a'+i))})
	}
	return seedEndpoint(t, svc, providerID, label, domain.UpstreamAuthAPIKey, inputs...)
}

func keyIDsOf(t *testing.T, store *memEndpointStore, endpointID string) []string {
	t.Helper()
	endpoint := mustStored(t, store, endpointID)
	ids := make([]string, 0, len(endpoint.Keys()))
	for _, key := range endpoint.Keys() {
		ids = append(ids, key.ID())
	}
	return ids
}

func refForKey(rows []BulkRowResult, keyID string) BulkRowResult {
	for _, row := range rows {
		if row.ID == keyID {
			return row
		}
	}
	return BulkRowResult{ID: "absent", Error: domain.NewNotFoundError("no row for " + keyID)}
}

func TestBulkKeyStatusParksEveryCredentialTheSelectorNames(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	endpoint := seedKeyedAccount(t, svc, "deepseek", "acct", 3)
	ids := keyIDsOf(t, store, endpoint.ID())

	outcome, err := svc.BulkKeyStatus(context.Background(), BulkSelector{IDs: ids[:2]}, "disabled", 0, 2)
	if err != nil {
		t.Fatalf("BulkKeyStatus() error = %v", err)
	}
	if outcome.Matched != 2 || outcome.Changed != 2 {
		t.Fatalf("outcome = %+v, want both named keys moved", outcome)
	}
	stored := mustStored(t, store, endpoint.ID())
	parked := 0
	for _, key := range stored.Keys() {
		if key.Status() == domain.UpstreamKeyDisabled {
			parked++
		}
	}
	if parked != 2 {
		t.Fatalf("%d key(s) disabled, want the two the selector named", parked)
	}
}

// TestBulkKeyStatusRefusesTheRowThatWouldEmptyTheAccount is F11 stated over a set rather
// than per row: each of the three keys could be disabled alone and still leave the account
// usable, and it is the walk across all of them that makes the last one a refusal.
func TestBulkKeyStatusRefusesTheRowThatWouldEmptyTheAccount(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	endpoint := seedKeyedAccount(t, svc, "deepseek", "acct", 3)
	ids := keyIDsOf(t, store, endpoint.ID())

	outcome, err := svc.BulkKeyStatus(context.Background(), BulkSelector{IDs: ids}, "disabled", 0, 3)
	if err != nil {
		t.Fatalf("BulkKeyStatus() error = %v", err)
	}
	refused := 0
	for _, row := range outcome.Results {
		if row.Error != nil {
			refused++
			if domain.AsAppError(row.Error).Code != "CONFLICT" {
				t.Fatalf("refusal code = %q, want CONFLICT", domain.AsAppError(row.Error).Code)
			}
		}
	}
	if refused != 1 {
		t.Fatalf("refused rows = %d, want exactly the one disable that would empty the account", refused)
	}
	if outcome.Changed != 2 {
		t.Fatalf("changed = %d, want the two disables that left one credential standing", outcome.Changed)
	}
	usable := 0
	for _, key := range mustStored(t, store, endpoint.ID()).Keys() {
		if key.Available(testNow) {
			usable++
		}
	}
	if usable != 1 {
		t.Fatalf("%d credential(s) still usable, want the floor the account owns", usable)
	}
}

// TestBulkKeyStatusOnANoAuthAccountHasNoFloor pins the other side of the same rule: an
// account that authenticates without a stored credential has no key to protect, so a mass
// disable there is not a refusal.
func TestBulkKeyStatusOnANoAuthAccountHasNoFloor(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("opencode"), newTestSealer(t), nil)
	endpoint, err := domain.NewUpstreamEndpoint("ep_noauth", "opencode", "acct", domain.UpstreamAuthNone, 1, testNow)
	if err != nil {
		t.Fatalf("building the account: %v", err)
	}
	key, err := domain.NewUpstreamKey("spare", endpoint.ID(), "uky_spare", "sealed", "sk-…cret", 1, testNow)
	if err != nil {
		t.Fatalf("building the credential: %v", err)
	}
	endpoint.AttachKey(key)
	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("storing the account: %v", err)
	}

	outcome, err := svc.BulkKeyStatus(context.Background(), BulkSelector{All: true}, "disabled", 0, 1)
	if err != nil {
		t.Fatalf("BulkKeyStatus() error = %v", err)
	}
	if outcome.Changed != 1 || outcome.Results[0].Error != nil {
		t.Fatalf("outcome = %+v, want a no-auth account's spare credential disabled", outcome)
	}
}

func TestBulkKeyStatusWindowFollowsTheParentProvider(t *testing.T) {
	store := newMemEndpointStore()
	svc := newTestEndpointService(t, store, newFakeIndex("gemini", "deepseek"), newTestSealer(t), nil)
	svc.health = &stubKeyHealthPolicy{policies: map[string]domain.KeyHealthPolicy{
		"gemini": {QuotaWindow: 5 * time.Hour},
	}}
	gemini := seedKeyedAccount(t, svc, "gemini", "gemini-acct", 2)
	deepseek := seedKeyedAccount(t, svc, "deepseek", "deepseek-acct", 2)

	// One credential per account, never the last: an account keeps a floor, so parking
	// both would be refused and the window the test measures would never be written.
	ids := []string{keyIDsOf(t, store, gemini.ID())[0], keyIDsOf(t, store, deepseek.ID())[0]}
	if _, err := svc.BulkKeyStatus(context.Background(), BulkSelector{IDs: ids}, "disabled", 0, 2); err != nil {
		t.Fatalf("BulkKeyStatus() error = %v", err)
	}
	for endpointID, want := range map[string]time.Duration{gemini.ID(): 5 * time.Hour, deepseek.ID(): 24 * time.Hour} {
		key := mustStored(t, store, endpointID).Keys()[0]
		until := key.DisabledUntil()
		if key.ID() != ids[0] && key.ID() != ids[1] {
			continue
		}
		if until == nil || !until.Equal(testNow.Add(want)) {
			t.Fatalf("%s window = %v, want %v from its own provider", endpointID, until, want)
		}
	}
}

func TestBulkKeyStatusRefusesAStaleCountWithoutTouchingAnyKey(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	endpoint := seedKeyedAccount(t, svc, "deepseek", "acct", 2)
	ids := keyIDsOf(t, store, endpoint.ID())

	_, err := svc.BulkKeyStatus(context.Background(), BulkSelector{IDs: ids}, "disabled", 0, 1)
	if domain.AsAppError(err).Code != "BULK_TARGET_COUNT_MISMATCH" {
		t.Fatalf("code = %q, want the count the caller confirmed to be checked first", domain.AsAppError(err).Code)
	}
	for _, key := range mustStored(t, store, endpoint.ID()).Keys() {
		if key.Status() != domain.UpstreamKeyActive {
			t.Fatal("a refused bulk key action moved a credential anyway")
		}
	}
}

func TestBulkKeyTagsResolveOnTheCredentialNotTheAccount(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	endpoint := seedKeyedAccount(t, svc, "deepseek", "acct", 2)
	ids := keyIDsOf(t, store, endpoint.ID())
	if _, err := svc.BulkKeyTags(context.Background(), BulkSelector{IDs: ids[:1]},
		[]string{"event-week"}, nil, false, 1); err != nil {
		t.Fatalf("BulkKeyTags() error = %v", err)
	}

	// The account itself holds no tag, and a tag selector on the credential still reaches
	// it: `all` would mean every credential in scope, not every credential wearing the
	// label, so naming the tag is the only selector that means what the operator saw.
	outcome, err := svc.BulkKeyTags(context.Background(), BulkSelector{Tag: "event-week"},
		nil, []string{"event-week"}, false, 1)
	if err != nil {
		t.Fatalf("BulkKeyTags() removal error = %v", err)
	}
	if outcome.Matched != 1 || outcome.Changed != 1 {
		t.Fatalf("outcome = %+v, want the one tagged credential", outcome)
	}
	if ref := refForKey(outcome.Results, ids[0]); ref.Error != nil {
		t.Fatalf("the credential the removal targeted = %+v, want it answered as a landed row", ref)
	}
	for _, key := range mustStored(t, store, endpoint.ID()).Keys() {
		if key.ID() != ids[1] && len(key.Tags()) != 0 {
			t.Fatalf("%s tags = %v, want the named label gone", key.ID(), key.Tags())
		}
	}
}

func TestBulkKeyDeleteRemovesTheSetAndRefusesTheLastCredential(t *testing.T) {
	svc, store := newEndpointSvc(t, "deepseek")
	endpoint := seedKeyedAccount(t, svc, "deepseek", "acct", 2)
	ids := keyIDsOf(t, store, endpoint.ID())

	outcome, err := svc.BulkKeyDelete(context.Background(), BulkSelector{IDs: ids}, 2)
	if err != nil {
		t.Fatalf("BulkKeyDelete() error = %v", err)
	}
	if outcome.Changed != 1 {
		t.Fatalf("changed = %d, want one credential gone", outcome.Changed)
	}
	if ref := refForKey(outcome.Results, ids[1]); ref.Error == nil ||
		domain.AsAppError(ref.Error).Code != "CONFLICT" {
		t.Fatalf("row for the surviving credential = %+v, want the CONFLICT that protects the account", ref)
	}
	if left := mustStored(t, store, endpoint.ID()).Keys(); len(left) != 1 {
		t.Fatalf("%d credential(s) left, want the one the floor kept", len(left))
	}
}
