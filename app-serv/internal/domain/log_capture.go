// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/log_capture.go
// @for       The capture, truncation, and retention rules that decide whether a
//
//	byte is stored and for how long.
//
// @uses      internal/domain (AppError constructors), strings, time.
// @reason    SPEC-API-001 §7.13 makes body storage conditional on a setting and
//
//	truncated to a configured size, with retention deleting older rows.
//	Both rules decide what is persisted, so they live here where a unit
//	test can exercise the boundary with no database.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"strings"
	"time"
)

// CaptureBodies applies the §7.13 capture rule: with capture off both bodies
// are dropped, and with it on each is truncated to maxBytes with a marker, so a
// reader can tell a truncated payload from a short one. A maxBytes below one is
// treated as "store nothing rather than everything": the failure mode of
// guessing must be storing less than intended, never more.
func CaptureBodies(requestBody, responseBody string, enabled bool, maxBytes int) (string, string) {
	if !enabled || maxBytes < 1 {
		return "", ""
	}
	return TruncateBody(requestBody, maxBytes), TruncateBody(responseBody, maxBytes)
}

// TruncateBody cuts a body to at most maxBytes bytes of its original text and
// appends the marker. The cut is on a byte boundary, which can split a UTF-8
// sequence: the stored body is opaque text a viewer renders verbatim, and
// re-encoding around a cut would rewrite the payload the operator needs to see.
// The marker is excluded from the byte budget, so a truncated body is always
// slightly longer than maxBytes and never indistinguishable from an untruncated
// one of exactly that size.
func TruncateBody(body string, maxBytes int) string {
	if maxBytes < 1 {
		return ""
	}
	if len(body) <= maxBytes {
		return body
	}
	return body[:maxBytes] + TruncationMarker
}

// Truncated reports whether the stored body carries the truncation marker,
// which is how the read side tells a viewer where the cut happened.
func Truncated(body string) bool {
	return strings.HasSuffix(body, TruncationMarker)
}

// RetentionCutoff is the instant before which a row is expired: a row strictly
// older than retentionDays is deleted, so the boundary itself survives. A
// non-positive retention is rejected by the settings validator before it
// reaches here; this clamps to zero days rather than deleting everything, since
// deleting the whole log on a bad setting is the worse failure.
func RetentionCutoff(now time.Time, retentionDays int) time.Time {
	if retentionDays < 0 {
		retentionDays = 0
	}
	return now.UTC().AddDate(0, 0, -retentionDays)
}

// ErrRequestLogNotFound is the sentinel a missing log row maps to.
var ErrRequestLogNotFound = NewNotFoundError("request log not found")
