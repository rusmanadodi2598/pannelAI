// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/key_use.go
// @for       The §7.3 key-usage rule: every authenticated data-plane call
//
//	advances the presenting key's counters.
//
// @uses      internal/domain, context, time.
// @reason    SPEC-API-001 §7.3 exposes request_count and last_used_at on every
//
//	gateway key, and the register's D3 decision (2026-09-19) is that the
//	counter follows every authenticated data-plane call. The write runs
//	where the §4 rule is decided — ChatService.Authenticate — because that
//	is the one place chat, models, media, and embeddings all ask whether a
//	key may proceed, so one call site counts for every route instead of
//	whichever routes remember to count.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// KeyUseRecorder applies one authenticated call to the key that presented the
// credential. The gateway key repository implements it.
type KeyUseRecorder interface {
	// RecordUse advances the key's request counter and stamps last_used_at.
	// A missing row must yield domain.ErrGatewayKeyNotFound.
	RecordUse(ctx context.Context, id string, usedAt time.Time) error
}

// recordKeyUse applies one admitted call to the key's counters.
//
// A failed write is deliberately not returned: the key was valid and the
// request it authorised is already on its way, so refusing the call over a
// counter would turn bookkeeping into an outage. The next call retries the
// write, and a key whose counter lags is a reporting inaccuracy rather than a
// served request the client cannot act on.
func (s *ChatService) recordKeyUse(ctx context.Context, key domain.GatewayKey) {
	if s.keyUse == nil {
		return
	}
	// reason: a counter write must not fail an authenticated request; the
	// repository returns domain.ErrGatewayKeyNotFound for a row that vanished,
	// which the next call of the same key reports again.
	_ = s.keyUse.RecordUse(ctx, key.ID(), s.clock().UTC())
}
