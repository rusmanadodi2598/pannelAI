// Package handler holds the HTTP surface of app-bot.
//
// @file      internal/handler/webhook_latency_test.go
// @for       The 300 ms acknowledgement budget of the webhook route.
// @uses      testing, net/http, net/http/httptest, strings, time, logx.
// @reason    Telegram retries any webhook slower than its own timeout, so the ack budget is a contract with the platform: the handler must queue and answer, never generate inline.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-21
package handler_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/logx"
)

const ackBudget = 300 * time.Millisecond

func TestWebhook_GivenQueuedUpdate_ThenAckStaysUnder300ms(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "start command", body: privateMessage(21, "/start")},
		{name: "help command", body: privateMessage(22, "/help")},
		{name: "plain text", body: privateMessage(23, "halo bot")},
		{name: "callback query", body: `{"update_id":24,"callback_query":{"id":"cb-1","from":{"id":42,"is_bot":false,"first_name":"Dodi"},"chat_instance":"ci-1","data":"draft:abc","message":{"message_id":11,"chat":{"id":42,"type":"private"},"date":1,"text":"preview"}}}`},
		{name: "edited message", body: `{"update_id":25,"edited_message":{"message_id":9,"from":{"id":42,"is_bot":false,"first_name":"Dodi"},"chat":{"id":42,"type":"private"},"date":1,"text":"halo"}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			webhook := handler.NewWebhook(webhookSecret, &fakeIntake{accepted: true}, logx.New("error", "test", io.Discard))
			req := httptest.NewRequest(http.MethodPost, "/internal/telegram/webhook", strings.NewReader(tc.body))
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", webhookSecret)
			rec := httptest.NewRecorder()

			started := time.Now()
			webhook.ServeHTTP(rec, req)
			elapsed := time.Since(started)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
			}
			if elapsed >= ackBudget {
				t.Fatalf("ack took %s, want under %s", elapsed, ackBudget)
			}
		})
	}
}
