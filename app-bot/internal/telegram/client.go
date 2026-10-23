// Package telegram adapts Telegram models to the service schema.
//
// @file      internal/telegram/client.go
// @for       The outbound Bot API client and its failure classification.
// @uses      context, errors, fmt, time, github.com/go-telegram/bot.
// @reason    One wrapper keeps the library at the boundary, and the class it returns is what lets the worker retry a rate limit but dead letter a blocked user instead of hammering both.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package telegram

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
)

// SendError carries the failure class of one send: a permanent failure must not
// be retried, and a rate limit states how long Telegram asked us to wait.
type SendError struct {
	permanent  bool
	retryAfter time.Duration
	err        error
}

// Error keeps the wrapped cause readable in a log line.
func (e *SendError) Error() string { return fmt.Sprintf("sending Telegram message: %v", e.err) }

// Unwrap exposes the library error to errors.Is and errors.As.
func (e *SendError) Unwrap() error { return e.err }

// Permanent reports whether retrying the same call can ever succeed.
func (e *SendError) Permanent() bool { return e.permanent }

// RetryAfter is the wait Telegram asked for, zero when it asked for none.
func (e *SendError) RetryAfter() time.Duration { return e.retryAfter }

// Client sends messages through the Bot API.
type Client struct {
	api *bot.Bot
}

// NewClient builds the client. An empty base keeps the library default.
func NewClient(token, apiBase string) (*Client, error) {
	options := make([]bot.Option, 0, 1)
	if apiBase != "" {
		options = append(options, bot.WithServerURL(apiBase))
	}
	api, err := bot.New(token, options...)
	if err != nil {
		return nil, fmt.Errorf("creating the Telegram client: %w", err)
	}
	return &Client{api: api}, nil
}

// SendMessage delivers plain text to one chat, with failures classified so the
// caller can tell a retryable outage from a permanent refusal.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	_, err := c.api.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
	if err == nil {
		return nil
	}
	return classify(err)
}

// classify maps a library error to the retry contract in SPEC-BOT-001 §12.1: a
// rate limit is retryable and carries its wait, a 4xx refusal is permanent, and
// everything else (network, 5xx) stays retryable.
func classify(err error) error {
	sendErr := &SendError{err: err}

	var rateLimited *bot.TooManyRequestsError
	if errors.As(err, &rateLimited) {
		sendErr.retryAfter = time.Duration(rateLimited.RetryAfter) * time.Second
		return sendErr
	}

	switch {
	case errors.Is(err, bot.ErrorBadRequest),
		errors.Is(err, bot.ErrorForbidden),
		errors.Is(err, bot.ErrorUnauthorized),
		errors.Is(err, bot.ErrorNotFound),
		bot.IsMigrateError(err):
		sendErr.permanent = true
	}
	return sendErr
}
