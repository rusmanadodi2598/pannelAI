// Package worker consumes queued work with bounded concurrency.
//
// @file      internal/worker/reply.go
// @for       The P2 reply copy for each work item kind.
// @uses      strings, internal/schema.
// @reason    The reply text is the only thing a user receives in P2, so it lives beside the consumer while the delivery loop keeps its own file.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-21
package worker

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/schema"
)

// Reply is the P2 copy. It states what the bot does now and promises nothing
// that is not built yet.
func Reply(item schema.WorkItem) string {
	text := strings.TrimSpace(item.Text)
	switch {
	case text == "/start" || strings.HasPrefix(text, "/start "):
		return "Halo, ini KENTANG TECH Media.\n\nBot aktif dan pesanmu sudah diterima.\nGenerator image dan video masuk tahap berikutnya.\nKetik /help untuk daftar perintah."
	case text == "/help":
		return "Perintah yang aktif di fase ini:\n/start  mulai dan daftar akun\n/help   bantuan ini\n\nCredit, plan, dan generate menyusul di fase P3-P8."
	default:
		if item.Kind == schema.KindCallback {
			return "Tombol diterima. Aksi generate belum aktif di fase ini."
		}
		return "Pesan diterima. Generate image dan video belum aktif di fase ini."
	}
}
