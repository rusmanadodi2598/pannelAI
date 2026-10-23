// Package handler holds the HTTP surface of app-bot.
//
// @file      internal/handler/webhook_body_test.go
// @for       Body shape and acknowledgement deadline of the webhook route.
// @uses      testing, context, io, net/http, net/http/httptest, strings, time, logx, schema.
// @reason    A padded body and a slow queue are the two ways a webhook misses its contract, so both are pinned: one document only, and a bounded wait for the queue.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-22
package handler_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

// blockingIntake waits for its context, like a Redis write that never returns.
type blockingIntake struct{}

func (blockingIntake) Accept(ctx context.Context, _ schema.WorkItem) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
}

func TestWebhook_GivenPaddedBody_ThenOnlyOneDocumentIsAccepted(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "a second document is refused",
			body:       privateMessage(41, "halo") + privateMessage(42, "halo"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "trailing bytes are refused",
			body:       privateMessage(43, "halo") + "  trailing",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "one document with trailing whitespace is accepted",
			body:       privateMessage(44, "halo") + "\n",
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			webhook := handler.NewWebhook(webhookSecret, &fakeIntake{accepted: true}, logx.New("error", "test", io.Discard))
			req := httptest.NewRequest(http.MethodPost, "/internal/telegram/webhook", strings.NewReader(tc.body))
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", webhookSecret)
			rec := httptest.NewRecorder()

			webhook.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestWebhook_GivenOversizedPaddedBody_ThenTheReadingStops(t *testing.T) {
	webhook := handler.NewWebhook(webhookSecret, &fakeIntake{accepted: true}, logx.New("error", "test", io.Discard))
	body := privateMessage(45, "halo") + `{"padding":"` + strings.Repeat("a", 1<<20) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/telegram/webhook", strings.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", webhookSecret)
	rec := httptest.NewRecorder()

	webhook.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 413 or 400", rec.Code)
	}
}

func TestWebhook_GivenStalledQueue_ThenTheAckIsNotHeldPastTheBudget(t *testing.T) {
	webhook := handler.NewWebhook(webhookSecret, blockingIntake{}, logx.New("error", "test", io.Discard))
	req := httptest.NewRequest(http.MethodPost, "/internal/telegram/webhook", strings.NewReader(privateMessage(46, "halo")))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", webhookSecret)
	rec := httptest.NewRecorder()

	started := time.Now()
	webhook.ServeHTTP(rec, req)
	elapsed := time.Since(started)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if elapsed >= time.Second {
		t.Fatalf("ack took %s, want the 250 ms intake deadline to end the wait", elapsed)
	}
}
