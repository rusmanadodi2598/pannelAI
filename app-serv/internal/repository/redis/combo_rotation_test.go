// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/combo_rotation_test.go
// @for       Tests for the rotation store's key derivation and its no-Redis
//
//	paths.
//
// @uses      context, testing, internal/domain.
// @reason    The store's atomic step is one INCR inside a Lua script, which only
//
//	a live server can execute; what a unit test can and must pin is the
//	key naming (a collision would rotate the wrong combo) and the
//	short-list path that never touches Redis. The distribution rule
//	itself is tested in internal/domain, against the same function the
//	store calls.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package redisrepo

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestComboRotationKey pins the key shape: namespaced, and derived by hash so a
// combo name an operator typed cannot collide with another key's namespace.
func TestComboRotationKey(t *testing.T) {
	cases := []struct {
		name     string
		comboKey string
		sameAs   string
		distinct string
	}{
		{name: "a plain name", comboKey: "daily", distinct: "daily2"},
		{name: "a name with the namespace separator", comboKey: "pannelai:auth:session:x", distinct: "daily"},
		{name: "a name with a slash", comboKey: "a/b", distinct: "a-b"},
		{name: "an empty name", comboKey: "", distinct: "daily"},
		{name: "a name differing only in case", comboKey: "Daily", distinct: "daily"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := comboRotationKey(tc.comboKey)
			if !strings.HasPrefix(key, comboRotationPrefix) {
				t.Fatalf("comboRotationKey(%q) = %q, want the %q prefix", tc.comboKey, key, comboRotationPrefix)
			}
			if key == comboRotationPrefix {
				t.Fatalf("comboRotationKey(%q) carries no name part", tc.comboKey)
			}
			if again := comboRotationKey(tc.comboKey); again != key {
				t.Fatalf("comboRotationKey(%q) is not deterministic: %q then %q", tc.comboKey, key, again)
			}
			if other := comboRotationKey(tc.distinct); other == key {
				t.Fatalf("comboRotationKey(%q) and comboRotationKey(%q) collide on %q", tc.comboKey, tc.distinct, key)
			}
			// The name itself must not appear: a colon in a typed name would
			// otherwise let it impersonate another key's namespace.
			if tc.comboKey != "" && strings.Contains(key, tc.comboKey) {
				t.Fatalf("comboRotationKey(%q) = %q, want the name hashed, not embedded", tc.comboKey, key)
			}
		})
	}
}

// TestComboRotationStore_ShortListSkipsRedis proves a list with nothing to
// distribute returns its stored order without a round trip, so a one-model combo
// keeps working when Redis is unreachable.
func TestComboRotationStore_ShortListSkipsRedis(t *testing.T) {
	// A nil client is the point: any call would panic, and none may happen.
	store := NewComboRotationStore(nil)
	cases := []struct {
		name   string
		models []string
	}{
		{name: "an empty list", models: []string{}},
		{name: "one model", models: []string{"a/one"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order, err := store.Next(context.Background(), "daily", tc.models, 1)
			if err != nil {
				t.Fatalf("Next() error = %v, want no Redis call for a short list", err)
			}
			if !reflect.DeepEqual(order, tc.models) {
				t.Fatalf("Next() = %v, want the stored order %v", order, tc.models)
			}
		})
	}
}

// TestComboRotationStore_ReturnedOrderMatchesTheDomainRule pins the contract the
// store owes its caller: whatever the counter is, the order it returns is the
// order domain.RotationRequestIndex and domain.RotateRefs produce. The assertion
// runs without Redis by deriving the index the way the store does.
func TestComboRotationStore_ReturnedOrderMatchesTheDomainRule(t *testing.T) {
	models := []string{"a/one", "b/two", "c/three"}
	for _, sticky := range []int{1, 2, 3} {
		state := domain.RotationState{}
		for request := range 12 {
			order, next := domain.ComboRoundRobin.NextOrder(models, sticky, state)
			state = next
			derived := domain.RotateRefs(models, domain.RotationRequestIndex(request, sticky, len(models)))
			if !reflect.DeepEqual(derived, order) {
				t.Fatalf("sticky=%d request=%d: store path gives %v, NextOrder gives %v", sticky, request, derived, order)
			}
		}
	}
}
