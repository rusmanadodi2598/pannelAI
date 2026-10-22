// Package telegram adapts Telegram models to the service schema.
//
// @file      internal/telegram/update_test.go
// @for       Update-to-work-item mapping rules.
// @uses      testing, models, schema.
// @reason    Telegram sends many update kinds the bot must ignore without failing, so the mapping table is the contract for what enters the queue.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package telegram_test

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/telegram"
)

func TestMap_GivenUpdateKinds_ThenOnlyPrivateChatsBecomeWork(t *testing.T) {
	privateChat := models.Chat{ID: 42, Type: models.ChatTypePrivate}
	groupChat := models.Chat{ID: -100, Type: models.ChatTypeGroup}
	from := &models.User{ID: 42, Username: "dodi"}

	cases := []struct {
		name     string
		update   models.Update
		wantOK   bool
		wantKind string
		wantText string
	}{
		{
			name:   "private message is mapped",
			update: models.Update{ID: 1, Message: &models.Message{ID: 7, From: from, Chat: privateChat, Text: "/start"}},
			wantOK: true, wantKind: schema.KindMessage, wantText: "/start",
		},
		{
			name:   "group message is ignored",
			update: models.Update{ID: 2, Message: &models.Message{ID: 8, From: from, Chat: groupChat, Text: "halo"}},
			wantOK: false,
		},
		{
			name:   "edited message is ignored",
			update: models.Update{ID: 3, EditedMessage: &models.Message{ID: 9, From: from, Chat: privateChat, Text: "edit"}},
			wantOK: false,
		},
		{
			name:   "message without a sender is ignored",
			update: models.Update{ID: 4, Message: &models.Message{ID: 10, Chat: privateChat, Text: "halo"}},
			wantOK: false,
		},
		{
			name: "private callback is mapped",
			update: models.Update{ID: 5, CallbackQuery: &models.CallbackQuery{
				ID: "cb-1", From: models.User{ID: 42, Username: "dodi"}, Data: "draft:1",
				Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: 11, Chat: privateChat}},
			}},
			wantOK: true, wantKind: schema.KindCallback,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item, ok, err := telegram.Map(tc.update)
			if err != nil {
				t.Fatalf("Map() error = %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if item.Kind != tc.wantKind {
				t.Fatalf("kind = %q, want %q", item.Kind, tc.wantKind)
			}
			if tc.wantText != "" && item.Text != tc.wantText {
				t.Fatalf("text = %q, want %q", item.Text, tc.wantText)
			}
			if item.ChatID != 42 || item.UserID != 42 {
				t.Fatalf("ids = %d/%d, want 42/42", item.ChatID, item.UserID)
			}
		})
	}
}
