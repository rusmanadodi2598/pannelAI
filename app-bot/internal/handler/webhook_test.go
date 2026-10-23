// Package handler holds the HTTP surface of app-bot.
//
// @file      internal/handler/webhook_test.go
// @for       Telegram webhook intake contract tests.
// @uses      testing, net/http/httptest, strings, errors, logx.
// @reason    The webhook is the only public write path and Telegram retries it, so secret, duplicate, and ignored-update behavior are pinned before the handler exists.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-21
package handler_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

const webhookSecret = "test-secret-1234567890"

type fakeIntake struct {
	got      []schema.WorkItem
	accepted bool
	err      error
}

func (f *fakeIntake) Accept(_ context.Context, item schema.WorkItem) (bool, error) {
	f.got = append(f.got, item)
	if f.err != nil {
		return false, f.err
	}
	return f.accepted, nil
}

func privateMessage(updateID int64, text string) string {
	return `{"update_id":` + intString(updateID) + `,"message":{"message_id":7,"from":{"id":42,"is_bot":false,"first_name":"Dodi","username":"dodi"},"chat":{"id":42,"type":"private"},"date":1,"text":"` + text + `"}}`
}

func TestWebhook_GivenRequestVariations_ThenStatusAndIntakeCallsAreCorrect(t *testing.T) {
	cases := []struct {
		name       string
		secret     string
		body       string
		intake     *fakeIntake
		wantStatus int
		wantCalls  int
		wantText   string
	}{
		{name: "missing secret is refused", secret: "", body: privateMessage(1, "/start"), intake: &fakeIntake{accepted: true}, wantStatus: http.StatusForbidden},
		{name: "wrong secret is refused", secret: "wrong-secret-000000000", body: privateMessage(2, "/start"), intake: &fakeIntake{accepted: true}, wantStatus: http.StatusForbidden},
		{name: "valid message is queued", secret: webhookSecret, body: privateMessage(3, "/start"), intake: &fakeIntake{accepted: true}, wantStatus: http.StatusOK, wantCalls: 1, wantText: "/start"},
		{name: "duplicate is acknowledged without a second enqueue", secret: webhookSecret, body: privateMessage(4, "halo"), intake: &fakeIntake{accepted: false}, wantStatus: http.StatusOK, wantCalls: 1},
		{name: "group message is ignored", secret: webhookSecret, body: `{"update_id":5,"message":{"message_id":8,"from":{"id":42,"is_bot":false,"first_name":"Dodi"},"chat":{"id":-100,"type":"group"},"date":1,"text":"halo"}}`, intake: &fakeIntake{accepted: true}, wantStatus: http.StatusOK},
		{name: "malformed body is refused", secret: webhookSecret, body: `{"update_id":`, intake: &fakeIntake{accepted: true}, wantStatus: http.StatusBadRequest},
		{name: "intake failure answers 500", secret: webhookSecret, body: privateMessage(6, "halo"), intake: &fakeIntake{err: errors.New("redis down")}, wantStatus: http.StatusInternalServerError, wantCalls: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			webhook := handler.NewWebhook(webhookSecret, tc.intake, logx.New("error", "test", io.Discard))
			req := httptest.NewRequest(http.MethodPost, "/internal/telegram/webhook", strings.NewReader(tc.body))
			if tc.secret != "" {
				req.Header.Set("X-Telegram-Bot-Api-Secret-Token", tc.secret)
			}
			rec := httptest.NewRecorder()

			webhook.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if len(tc.intake.got) != tc.wantCalls {
				t.Fatalf("intake calls = %d, want %d", len(tc.intake.got), tc.wantCalls)
			}
			if tc.wantText != "" && tc.intake.got[0].Text != tc.wantText {
				t.Fatalf("queued text = %q, want %q", tc.intake.got[0].Text, tc.wantText)
			}
		})
	}
}

func TestWebhook_GivenOversizedBody_ThenRequestIsRefused(t *testing.T) {
	webhook := handler.NewWebhook(webhookSecret, &fakeIntake{accepted: true}, logx.New("error", "test", io.Discard))
	body := strings.Repeat("a", 1<<20+1)
	req := httptest.NewRequest(http.MethodPost, "/internal/telegram/webhook", strings.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", webhookSecret)
	rec := httptest.NewRecorder()

	webhook.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 413 or 400", rec.Code)
	}
}

func intString(value int64) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 20)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
