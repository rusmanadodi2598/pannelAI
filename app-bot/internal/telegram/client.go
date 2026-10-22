// Package telegram adapts Telegram models to the service schema.
//
// @file      internal/telegram/client.go
// @for       The outbound Bot API client.
// @uses      context, fmt, github.com/go-telegram/bot.
// @reason    One wrapper keeps the library at the boundary, so services depend on a two-method interface and a test can assert the send without a network call.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
)

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

// SendMessage delivers plain text to one chat.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	_, err := c.api.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
	if err != nil {
		return fmt.Errorf("sending Telegram message: %w", err)
	}
	return nil
}
