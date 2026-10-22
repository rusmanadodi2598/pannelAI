// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_forced_stream_cap_test.go
// @for       The byte ceiling one forced-stream fold enforces before it retains another event.
// @uses      bytes, testing.
// @reason    The event count alone lets an upstream hold tens of gigabytes; the test pins the total-byte ceiling at a stream that stays within every per-event and count bound.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package dataplane

import (
	"bytes"
	"testing"
)

func TestReadFoldEventsRefusesMoreBytesThanTheFoldMayHold(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), maxEventBytes-64)
	var stream bytes.Buffer
	for written := 0; written <= maxFoldBytes; written += len(payload) {
		stream.WriteString("data: ")
		stream.Write(payload)
		stream.WriteString("\n\n")
	}

	if _, err := readFoldEvents(&stream); err == nil {
		t.Fatal("readFoldEvents accepted a fold larger than maxFoldBytes")
	}
}
