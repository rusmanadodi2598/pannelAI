// Package router wires the HTTP surface of app-serv onto one mux.
//
// @file      internal/router/router_deps_guard_test.go
// @for       Tests for the boot-time assertion over the router's dependencies.
// @uses      fmt, reflect, strings, testing, internal/handler.
// @reason    The guard is a hand-written list beside a struct it must not fall
//
//	behind, so the test enumerates Deps by reflection: a handler field
//	added without a check makes this fail rather than leaving a route
//	that can only be discovered by clicking it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-10-04
package router

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
)

// handlerFieldNames lists every Deps field a nil value can hide a broken
// deployment behind: pointers to handlers and the limiter interface.
func handlerFieldNames(t *testing.T) []string {
	t.Helper()
	kind := reflect.TypeOf(Deps{})
	names := make([]string, 0, kind.NumField())
	for i := range kind.NumField() {
		field := kind.Field(i)
		switch field.Type.Kind() {
		case reflect.Pointer, reflect.Interface:
			names = append(names, field.Name)
		}
	}
	if len(names) == 0 {
		t.Fatal("Deps carries no handler field, so the guard tests nothing")
	}
	return names
}

func TestAssertWiredGivenNothingWiredNamesEveryHandler(t *testing.T) {
	err := Deps{}.AssertWired()
	if err == nil {
		t.Fatal("AssertWired() = nil for an empty Deps, so a bare boot would pass")
	}
	reported := reportedNames(t, err.Error())
	wanted := handlerFieldNames(t)
	if len(reported) != len(wanted) {
		t.Fatalf("AssertWired() reported %v, want every Deps handler field %v", reported, wanted)
	}
	for _, name := range wanted {
		if !contains(reported, name) {
			t.Fatalf("AssertWired() message %q omits %q, so that handler is unchecked", err, name)
		}
	}
}

func TestAssertWiredGivenPartialWiringReportsOnlyTheRest(t *testing.T) {
	deps := Deps{System: handler.NewSystemHandler(handler.SystemHandlerDeps{}), Usage: &handler.UsageHandler{}}
	err := deps.AssertWired()
	if err == nil {
		t.Fatal("AssertWired() = nil while most handlers are missing")
	}
	reported := reportedNames(t, err.Error())
	for _, name := range []string{"System", "Usage"} {
		if contains(reported, name) {
			t.Fatalf("AssertWired() reports %q as missing although it is wired: %v", name, reported)
		}
	}
	for _, name := range []string{"Chat", "Quota", "RateLimiter"} {
		if !contains(reported, name) {
			t.Fatalf("AssertWired() omits %q from %v", name, reported)
		}
	}
}

// reportedNames reads the list out of the error message, so a field whose name
// is a prefix of another ("Usage" beside "UsageLive") cannot pass by accident.
func reportedNames(t *testing.T, message string) []string {
	t.Helper()
	head, list, found := strings.Cut(message, "are unwired: ")
	if !found {
		t.Fatalf("AssertWired() message %q does not end with the name list", message)
	}
	names := strings.Split(list, ", ")
	prefix := "router: " + fmt.Sprint(len(names)) + " dependency fields "
	if !strings.HasPrefix(head, prefix) {
		t.Fatalf("message %q counts a different number of fields than it lists (%d)", message, len(names))
	}
	return names
}

func contains(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
