// Package schema holds request and queue DTOs with their validation rules.
//
// @file      internal/schema/telegram.go
// @for       The compact work item the webhook queues for the worker.
// @uses      github.com/go-playground/validator/v10, internal/domain.
// @reason    Telegram updates carry fields the bot never uses, so the queue item is a typed subset validated before it is stored, not the raw update.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-21
package schema

import (
	"github.com/go-playground/validator/v10"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/domain"
)

const (
	KindMessage  = "message"
	KindCallback = "callback_query"
)

var validate = validator.New()

// WorkItem is one unit of queued Telegram work.
type WorkItem struct {
	UpdateID     int64  `json:"update_id" validate:"required"`
	ChatID       int64  `json:"chat_id" validate:"required"`
	UserID       int64  `json:"user_id" validate:"required"`
	Username     string `json:"username,omitempty" validate:"max=64"`
	Kind         string `json:"kind" validate:"required,oneof=message callback_query"`
	Text         string `json:"text,omitempty" validate:"max=4096"`
	CallbackData string `json:"callback_data,omitempty" validate:"max=64"`
	MessageID    int64  `json:"message_id,omitempty"`
}

// Validate refuses an item the worker cannot act on. The message is English
// and carries no field values, per AGENTS.md §1.3.
func (w WorkItem) Validate() error {
	if err := validate.Struct(w); err != nil {
		return domain.NewValidationError("invalid work item")
	}
	if w.Kind == KindMessage && w.Text == "" {
		return domain.NewValidationError("message text is required")
	}
	return nil
}
