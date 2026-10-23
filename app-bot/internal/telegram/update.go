// Package telegram adapts Telegram models to the service schema.
//
// @file      internal/telegram/update.go
// @for       Mapping one Telegram update onto a work item.
// @uses      github.com/go-telegram/bot/models, internal/schema.
// @reason    Telegram sends far more update kinds than the bot serves, so one mapping function decides what enters the queue and everything else is ignored without an error that would trigger a redelivery.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package telegram

import (
	"github.com/go-telegram/bot/models"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

// Map returns ok=false for updates the bot does not serve: group traffic,
// edits, channel posts, and non-text messages. P3 adds the media kinds.
func Map(update models.Update) (schema.WorkItem, bool, error) {
	if message := update.Message; message != nil {
		if message.Chat.Type != models.ChatTypePrivate {
			return schema.WorkItem{}, false, nil
		}
		if message.From == nil || message.Text == "" {
			return schema.WorkItem{}, false, nil
		}
		return schema.WorkItem{
			UpdateID:  update.ID,
			ChatID:    message.Chat.ID,
			UserID:    message.From.ID,
			Username:  message.From.Username,
			Kind:      schema.KindMessage,
			Text:      message.Text,
			MessageID: int64(message.ID),
		}, true, nil
	}

	if callback := update.CallbackQuery; callback != nil {
		if callback.Message.Message == nil || callback.Message.Message.Chat.Type != models.ChatTypePrivate {
			return schema.WorkItem{}, false, nil
		}
		return schema.WorkItem{
			UpdateID:     update.ID,
			ChatID:       callback.Message.Message.Chat.ID,
			UserID:       callback.From.ID,
			Username:     callback.From.Username,
			Kind:         schema.KindCallback,
			CallbackData: callback.Data,
			MessageID:    int64(callback.Message.Message.ID),
		}, true, nil
	}

	return schema.WorkItem{}, false, nil
}
