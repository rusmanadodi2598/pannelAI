// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/vision_rotation_test.go
// @for       Tests for the vision rotation key's derivation, which is the part
//
//	of the store a unit test can pin without a server.
//
// @uses      testing.
// @reason    The store's read and write are plain Redis calls, so what a unit
//
//	test can and must pin is the key naming: the rotation is one key
//	per deployment, and a derivation that collided with another
//	namespace would make the adapter share state with it. The
//	state's read-write behaviour is covered by the augmenter's tests
//	through the repository interface.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package redisrepo

import (
	"strings"
	"testing"
)

// TestVisionRotationKey pins the key shape: namespaced, hashed, and stable, so
// the one key the deployment holds is always the same one and belongs to this
// namespace alone.
func TestVisionRotationKey(t *testing.T) {
	key := visionRotationKey()

	if !strings.HasPrefix(key, visionRotationPrefix) {
		t.Fatalf("visionRotationKey() = %q, want the %q prefix", key, visionRotationPrefix)
	}
	if key == visionRotationPrefix {
		t.Fatal("visionRotationKey() carries no name part")
	}
	if again := visionRotationKey(); again != key {
		t.Fatalf("visionRotationKey() is not deterministic: %q then %q", key, again)
	}
	// Distinct from every other namespaced key this package derives: a shared
	// key would let the adapter's rotation advance with a combo's traffic.
	if other := comboRotationKey("daily"); other == key {
		t.Fatal("vision rotation key collides with the combo rotation namespace")
	}
	if session := sessionKey("anything"); session == key {
		t.Fatal("vision rotation key collides with the session namespace")
	}
}
