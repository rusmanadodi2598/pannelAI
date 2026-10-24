// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_refusal_test.go
// @for       What one refused request leaves in the log: the identifiers an operator needs, and no part of the vendor's own text.
// @uses      bytes, context, encoding/json, errors, fmt, log/slog, net/http, strings, testing, internal/domain, internal/registry, internal/requestctx.
// @reason    A request-shaped refusal is the failure that writes nothing anywhere else, so the line is the only evidence it existed, and the rule keeping it safe is the one that must not drift: the same reason the access log withholds the message is the reason this one does.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-19
package dataplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/requestctx"
)

// refusalResolution is the address a real refusal arrives under: the client's name for
// the model, and the code the vendor was actually asked for.
func refusalResolution() Resolution {
	return Resolution{
		Provider:   registry.Provider{ID: "qoder"},
		ModelID:    "qwen3.8-flash",
		UpstreamID: "qfmodel",
	}
}

// captureRefusalLog points one engine's logger at a buffer a test can read back.
func captureRefusalLog(t *testing.T, engine *Engine) *bytes.Buffer {
	t.Helper()
	logged := &bytes.Buffer{}
	engine.logger = slog.New(slog.NewJSONHandler(logged, nil))
	return logged
}

// refusalFields decodes the one line that was written, failing when there was not
// exactly one, so a test that asserts a field also asserts the line exists.
func refusalFields(t *testing.T, logged *bytes.Buffer) map[string]string {
	t.Helper()
	if logged.String() == "" {
		t.Fatal("no log line was written, want the refusal on the record")
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(logged.String())), &record); err != nil {
		t.Fatalf("decoding the logged line: %v", err)
	}
	fields := make(map[string]string, len(record))
	for key, value := range record {
		fields[key] = fmt.Sprint(value)
	}
	return fields
}

func TestEngine_RefusalLineNamesTheModelEndpointAndKey(t *testing.T) {
	engine, _, selection := parkedKeyEngine(t)
	logged := captureRefusalLog(t, engine)

	engine.logRequestRefusal(requestctx.With(context.Background(), "038AKJREFUSED"),
		refusalResolution(), selection,
		&UpstreamError{Status: http.StatusBadRequest, Message: "None flow nodes found for pt-SECRETTOKEN"})

	fields := refusalFields(t, logged)
	if fields["level"] != "WARN" {
		t.Fatalf("level = %q, want the refusal written at WARN", fields["level"])
	}
	for key, want := range map[string]string{
		"request_id":      "038AKJREFUSED",
		"provider_id":     "qoder",
		"model":           "qwen3.8-flash",
		"upstream_model":  "qfmodel",
		"endpoint_id":     selection.Endpoint.ID(),
		"key_id":          selection.Key.ID(),
		"upstream_status": "400",
	} {
		if fields[key] != want {
			t.Fatalf("%s = %q, want %q", key, fields[key], want)
		}
	}
	// The message is the client's to receive and nobody's to log: it is where a
	// vendor echoes a credential back, and the same rule the access log follows.
	if line := logged.String(); strings.Contains(line, "flow nodes") || strings.Contains(line, "pt-SECRETTOKEN") {
		t.Fatalf("the refusal leaked the vendor's text: %s", line)
	}
}

// TestEngine_RefusalLineNamesTheComboAndTheSignature pins the two fields that
// change the fix: a member of a combo is a different fault from a model the client
// addressed itself, and the signature says which read produced the status.
func TestEngine_RefusalLineNamesTheComboAndTheSignature(t *testing.T) {
	engine, _, selection := parkedKeyEngine(t)
	logged := captureRefusalLog(t, engine)
	resolution := refusalResolution()
	resolution.Combo = comboRow("qwen-pool", "qoder/qfmodel")

	engine.logRequestRefusal(context.Background(), resolution, selection,
		&UpstreamError{Status: http.StatusBadRequest, Message: "refused", SignatureID: "sig-qoder-refusal"})

	fields := refusalFields(t, logged)
	if fields["combo"] != "qwen-pool" {
		t.Fatalf("combo = %q, want the combo the client addressed", fields["combo"])
	}
	if fields["signature_id"] != "sig-qoder-refusal" {
		t.Fatalf("signature_id = %q, want the signature that read the refusal", fields["signature_id"])
	}
}

// TestEngine_RefusalLineStaysQuietOnWhatAlreadyParks keeps the line about the one
// class that is otherwise invisible. Every failure here already reaches the panel
// through the credential's health, and a line per failover attempt would bury the
// refusal in noise from the paths that need no help.
func TestEngine_RefusalLineStaysQuietOnWhatAlreadyParks(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "a throttle", err: &UpstreamError{Status: http.StatusTooManyRequests, Message: "slow down"}},
		{name: "a refused credential", err: &UpstreamError{Status: http.StatusUnauthorized, Message: "bad key"}},
		{name: "a spent account", err: &UpstreamError{Status: http.StatusPaymentRequired, Message: "no credit"}},
		{name: "a target without the model", err: &UpstreamError{Status: http.StatusNotFound, Message: "no such model"}},
		{name: "an upstream outage", err: &UpstreamError{Status: http.StatusInternalServerError, Message: "broke"}},
		{name: "a call that never reached the vendor", err: errors.New("dial tcp: connection refused")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine, _, selection := parkedKeyEngine(t)
			logged := captureRefusalLog(t, engine)

			engine.logRequestRefusal(context.Background(), refusalResolution(), selection, tc.err)

			if logged.String() != "" {
				t.Fatalf("wrote %q, want silence on a failure the health model already records", logged.String())
			}
		})
	}
}

// TestEngine_RecordFailureWritesTheRefusalAndParksNothing is the wiring proof: the
// line comes out of the path a real call walks, and writing it does not start
// recording the refusal as the credential's own failure.
func TestEngine_RecordFailureWritesTheRefusalAndParksNothing(t *testing.T) {
	engine, repo, selection := parkedKeyEngine(t)
	logged := captureRefusalLog(t, engine)
	before := repo.health[selection.Key.ID()]

	if action := engine.recordFailure(context.Background(), refusalResolution(), selection,
		&UpstreamError{Status: http.StatusBadRequest, Message: "None flow nodes found"}); action != domain.FailureActionStop {
		t.Fatalf("action = %q, want the chain stopped on a request-shaped refusal", action)
	}

	if logged.String() == "" {
		t.Fatal("the walk wrote no line for the refusal it stopped on")
	}
	if _, reason, _ := repo.outcome[selection.Endpoint.ID()].LastError(); reason != "the credential was refused" {
		t.Fatalf("recorded reason = %q, want the earlier park untouched by a refusal of the request", reason)
	}
	if after := repo.health[selection.Key.ID()]; after.ConsecutiveErrors() != before.ConsecutiveErrors() {
		t.Fatalf("error run = %d, want %d: refusing a request says nothing about the key",
			after.ConsecutiveErrors(), before.ConsecutiveErrors())
	}
}
