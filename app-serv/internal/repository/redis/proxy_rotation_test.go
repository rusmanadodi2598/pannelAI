// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/proxy_rotation_test.go
// @for       Tests for the proxy route store's key derivation and its
//
//	short-list path.
//
// @uses      context, reflect, strings, testing, internal/domain.
// @reason    The atomic steps (the INCR script and the TTL set) only execute on
//
//	a live server; what a unit test can and must pin is that the two key
//	families cannot collide with each other or with another namespace,
//	that derivation is deterministic, and that a pool of fewer than two
//	candidates is served without touching Redis (docs/PORT/
//	008-PORT-PROXY-ENGINE.md D5, D6).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-26
package redisrepo

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestProxyRotationKey(t *testing.T) {
	cases := []struct {
		name     string
		poolKey  string
		distinct string
	}{
		{name: "the global pool", poolKey: "global", distinct: "global2"},
		{name: "a key with the namespace separator", poolKey: "pannelai:proxy:rotation:x", distinct: "global"},
		{name: "a key differing only in case", poolKey: "Global", distinct: "global"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := proxyRotationKey(tc.poolKey)
			if !strings.HasPrefix(key, proxyRotationPrefix) {
				t.Fatalf("proxyRotationKey(%q) = %q, want the %q prefix", tc.poolKey, key, proxyRotationPrefix)
			}
			if key == proxyRotationPrefix {
				t.Fatalf("proxyRotationKey(%q) carries no pool part", tc.poolKey)
			}
			if again := proxyRotationKey(tc.poolKey); again != key {
				t.Fatalf("proxyRotationKey(%q) is not deterministic", tc.poolKey)
			}
			if other := proxyRotationKey(tc.distinct); other == key {
				t.Fatalf("proxyRotationKey(%q) and proxyRotationKey(%q) collide", tc.poolKey, tc.distinct)
			}
		})
	}
}

func TestProxyParkKey(t *testing.T) {
	park := proxyParkKey("prx_01HZZ9K2")
	rotation := proxyRotationKey("prx_01HZZ9K2")
	if !strings.HasPrefix(park, proxyParkPrefix) {
		t.Fatalf("proxyParkKey() = %q, want the %q prefix", park, proxyParkPrefix)
	}
	// The two families hash the same input: the prefixes alone must keep them
	// apart, because a parked id and a rotation counter answering on one key
	// would corrupt both rules at once.
	if park == rotation {
		t.Fatal("the park and rotation keys collide for the same id")
	}
	if again := proxyParkKey("prx_01HZZ9K2"); again != park {
		t.Fatal("proxyParkKey() is not deterministic")
	}
	if other := proxyParkKey("prx_other"); other == park {
		t.Fatal("two candidate ids collide on one cooldown key")
	}
}

func TestProxyRouteStore_ShortListSkipsRedis(t *testing.T) {
	// A store built with a nil client cannot answer a Redis call: reaching the
	// server from this path would panic rather than rotate, which is the
	// contract's own statement that one candidate needs no distribution.
	store := NewProxyRouteStore(nil)
	order, err := store.Next(context.Background(), "global", []string{"prx_one"})
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if !reflect.DeepEqual(order, []string{"prx_one"}) {
		t.Fatalf("Next() = %v, want the input unchanged", order)
	}
	order, err = store.Next(context.Background(), "global", nil)
	if err != nil {
		t.Fatalf("Next(nil pool) error = %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("Next(nil pool) = %v, want empty", order)
	}
}
