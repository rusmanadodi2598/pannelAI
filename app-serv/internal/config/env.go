// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/env.go
// @for       The environment-reading helpers and small validators the loader
//
//	uses.
//
// @uses      fmt, net/url, os, strconv, strings, time (standard library only).
// @reason    The unset/empty distinction is a rule of its own — a variable that
//
//	is set but malformed must fail the boot rather than fall back to a
//	default — and AGENTS.md §1.1 caps a file at 250 lines, so the helpers
//	live here and the loader reads as the field list it is.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// getenv returns the variable's value when the variable is set, even when that
// value is empty, and the fallback only when it is unset. Collapsing the two
// cases would make an explicitly empty HTTP_ADDR silently become ":8080",
// which is precisely the misconfiguration the boot validation exists to catch.
func getenv(key, fallback string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return v
}

// splitList reads a comma-separated environment value into its entries,
// dropping the blank ones so a trailing comma is not an entry of "".
func splitList(raw string) []string {
	entries := make([]string, 0)
	for _, entry := range strings.Split(raw, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

// isAbsoluteHTTPURL reports whether raw is an absolute http(s) URL, the only
// shape a browser may be redirected to and the only shape an OAuth provider
// will accept as a callback.
func isAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// isValidLogLevel reports whether the level names one slog supports. Validating
// here means a misspelled LOG_LEVEL is a boot failure rather than a value the
// logger silently ignores.
func isValidLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

// getenvInt reads an integer, using the fallback only when the variable is
// unset. A variable that is set but empty or malformed is an error: silently
// substituting a default would hide a broken deployment configuration.
func getenvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("not an integer: %q", v)
	}
	return n, nil
}

// getenvDuration reads a Go duration string with the same unset/empty rule.
func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("not a duration: %q", v)
	}
	return d, nil
}
