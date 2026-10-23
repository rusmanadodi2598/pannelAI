// Package handler holds the HTTP surface of app-bot.
//
// @file      internal/handler/webhook.go
// @for       The Telegram webhook intake route.
// @uses      context, crypto/subtle, encoding/json, errors, io, log/slog, net/http, time, internal/schema.
// @reason    Telegram retries any non-200 delivery, so this handler verifies the secret, bounds the body, dedupes, enqueues, and answers fast instead of doing generation work inline.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-21
package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/telegram"
)

const (
	webhookSecretHeader = "X-Telegram-Bot-Api-Secret-Token"
	webhookMaxBodyBytes = 1 << 20
	// acceptTimeout keeps the acknowledgement inside the 300 ms budget even
	// when Redis is slow. The handler answers 500 and Telegram retries, which
	// is better than holding the request until the server write timeout.
	acceptTimeout = 250 * time.Millisecond
)

// Intake accepts one queued work item.
type Intake interface {
	Accept(ctx context.Context, item schema.WorkItem) (bool, error)
}

// Webhook receives Telegram updates.
type Webhook struct {
	secret []byte
	intake Intake
	logger *slog.Logger
}

// NewWebhook wires the shared secret and the intake use case.
func NewWebhook(secret string, intake Intake, logger *slog.Logger) *Webhook {
	return &Webhook{secret: []byte(secret), intake: intake, logger: logger}
}

// ServeHTTP answers 200 for anything Telegram should not retry, including a
// duplicate and an update kind the bot does not serve.
func (h *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(h.secret) == 0 || subtle.ConstantTimeCompare([]byte(r.Header.Get(webhookSecretHeader)), h.secret) != 1 {
		h.logger.Warn("webhook secret rejected", "remote", r.RemoteAddr)
		writeBody(w, http.StatusForbidden, []byte("{\"code\":\"BOT_FORBIDDEN\",\"message\":\"forbidden\"}\n"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, webhookMaxBodyBytes)
	update, err := decodeUpdate(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeBody(w, http.StatusRequestEntityTooLarge, []byte("{\"code\":\"BOT_BODY_TOO_LARGE\",\"message\":\"request body is too large\"}\n"))
			return
		}
		writeBody(w, http.StatusBadRequest, []byte("{\"code\":\"BOT_BAD_REQUEST\",\"message\":\"invalid update body\"}\n"))
		return
	}

	item, ok, err := telegram.Map(update)
	if err != nil {
		writeBody(w, http.StatusBadRequest, []byte("{\"code\":\"BOT_BAD_REQUEST\",\"message\":\"invalid update body\"}\n"))
		return
	}
	if !ok {
		writeBody(w, http.StatusOK, []byte("{\"ok\":true}\n"))
		return
	}

	if err := h.accept(r, item); err != nil {
		h.logger.Error("webhook intake failed", "update_id", item.UpdateID, "error", err)
		writeBody(w, http.StatusInternalServerError, []byte("{\"code\":\"BOT_INTERNAL\",\"message\":\"internal server error\"}\n"))
		return
	}
	writeBody(w, http.StatusOK, []byte("{\"ok\":true}\n"))
}

// accept bounds the queue write to the acknowledgement budget.
func (h *Webhook) accept(r *http.Request, item schema.WorkItem) error {
	ctx, cancel := context.WithTimeout(r.Context(), acceptTimeout)
	defer cancel()
	_, err := h.intake.Accept(ctx, item)
	return err
}

// decodeUpdate reads exactly one update object and refuses a body that carries
// a second document or trailing bytes, so a padded body cannot slip past the
// size cap with only its first document inspected.
func decodeUpdate(body io.Reader) (models.Update, error) {
	var update models.Update
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&update); err != nil {
		return models.Update{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return models.Update{}, errTrailingData
		}
		return models.Update{}, err
	}
	return update, nil
}

// errTrailingData marks a body with more than one JSON document.
var errTrailingData = errors.New("trailing data after the update object")
