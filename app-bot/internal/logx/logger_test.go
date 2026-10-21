// Package logx writes structured logs with credential redaction.
//
// @file      internal/logx/logger_test.go
// @for       Table-driven tests that a credential attribute never reaches the log line.
// @uses      testing, bytes, encoding/json.
// @reason    OWASP A09 and AGENTS.md §1.6 require security-relevant logging without secrets, so redaction is pinned by tests rather than left to reviewer memory.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package logx_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/logx"
)

func TestNew_GivenSensitiveAttributes_ThenValuesAreRedacted(t *testing.T) {
	cases := []struct {
		name      string
		key       string
		value     string
		wantShown bool
	}{
		{name: "bot token is masked", key: "bot_token", value: "123456:AAH-secret"},
		{name: "client secret is masked", key: "client_secret", value: "s3cr3t"},
		{name: "authorization header is masked", key: "authorization", value: "Bearer abc"},
		{name: "signature is masked", key: "x_signature", value: "deadbeef"},
		{name: "plain field stays visible", key: "job_id", value: "job-42", wantShown: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := logx.New("info", "test", &buf)
			logger.Info("test event", tc.key, tc.value)

			line := buf.String()
			if strings.Contains(line, tc.value) && !tc.wantShown {
				t.Fatalf("log line leaked %q: %s", tc.value, line)
			}
			if tc.wantShown && !strings.Contains(line, tc.value) {
				t.Fatalf("log line dropped %q: %s", tc.value, line)
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(line), &decoded); err != nil {
				t.Fatalf("log line is not JSON: %v (%s)", err, line)
			}
			if decoded["service"] != "app-bot" {
				t.Fatalf("service = %v, want app-bot", decoded["service"])
			}
			if decoded["env"] != "test" {
				t.Fatalf("env = %v, want test", decoded["env"])
			}
		})
	}
}
