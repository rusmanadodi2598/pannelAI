// Package worker_test pins the consumer contract of app-bot.
//
// @file      internal/worker/reply_test.go
// @for       The P2 reply copy for each work item kind.
// @uses      testing, internal/schema, internal/worker.
// @reason    The reply text is the only thing a user sees in P2, so a change to it should fail a test rather than reach Telegram unnoticed.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-21
package worker_test

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/worker"
)

func TestReply_GivenWorkItemVariations_ThenCopyIsStable(t *testing.T) {
	cases := []struct {
		name         string
		item         schema.WorkItem
		wantContains string
	}{
		{name: "start", item: workItem("/start"), wantContains: "/help"},
		{name: "start with argument", item: workItem("/start referral"), wantContains: "KENTANG TECH Media"},
		{name: "help", item: workItem("/help"), wantContains: "Perintah yang aktif"},
		{name: "plain text", item: workItem("halo"), wantContains: "Pesan diterima."},
		{name: "callback", item: schema.WorkItem{Kind: schema.KindCallback, CallbackData: "draft:abc"}, wantContains: "Tombol diterima."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if reply := worker.Reply(tc.item); !strings.Contains(reply, tc.wantContains) {
				t.Fatalf("Reply() = %q, want it to contain %q", reply, tc.wantContains)
			}
		})
	}
}
