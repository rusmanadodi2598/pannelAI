// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/key_use_test.go
// @for       The §7.3 rule that an admitted data-plane call advances the
//
//	presenting key's request counter.
//
// @uses      internal/domain, context, errors, testing, time.
// @reason    The register's G6 decision (2026-09-19) is that `request_count`
//
//	follows every authenticated data-plane call, and the counter is
//	written where the §4 rule is decided — ChatService.Authenticate —
//	so one test covers chat, models, media, and embeddings at once. The
//	cases pin the three ways authentication ends (admitted, refused,
//	disabled) and the one way the write can fail.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubKeyLookup answers the authentication seam from memory.
type stubKeyLookup struct {
	key  domain.GatewayKey
	fail error
}

func (l *stubKeyLookup) GetByValueHash(_ context.Context, _ string) (domain.GatewayKey, error) {
	if l.fail != nil {
		return domain.GatewayKey{}, l.fail
	}
	return l.key, nil
}

// stubRequireKey answers the settings seam. err != nil models the settings
// store failing, which authentication must treat as "required" rather than
// skipping the check (deny by default).
type stubRequireKey struct {
	required bool
	err      error
}

func (s stubRequireKey) RequireAPIKey(context.Context) (bool, error) { return s.required, s.err }

// stubKeyUse collects the counter updates the data plane applies.
type stubKeyUse struct {
	ids  []string
	fail error
}

func (u *stubKeyUse) RecordUse(_ context.Context, id string, _ time.Time) error {
	u.ids = append(u.ids, id)
	return u.fail
}

// TestChatService_AuthenticateRecordsKeyUse pins when the counter moves: an
// admitted key advances it once, and a refused or disabled check does not.
func TestChatService_AuthenticateRecordsKeyUse(t *testing.T) {
	key := domain.NewGatewayKey("panel", "sk-live", "sk-…ive", time.Now().UTC())
	cases := []struct {
		name        string
		required    bool
		settingsErr error
		presented   string
		lookupFail  error
		wantRefused bool
		wantUses    int
	}{
		{name: "an admitted key", required: true, presented: "sk-live", wantUses: 1},
		{
			name: "an unknown key", required: true, presented: "sk-other",
			lookupFail: domain.ErrGatewayKeyNotFound, wantRefused: true,
		},
		{name: "no key presented", required: true, wantRefused: true},
		{name: "authentication disabled", required: false, wantUses: 0},
		{
			name:        "a settings read failure refuses even a valid key",
			settingsErr: errors.New("the settings store is down"), presented: "sk-live",
			wantRefused: true,
		},
		{
			name:        "a settings read failure refuses an absent key too",
			settingsErr: errors.New("the settings store is down"),
			wantRefused: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			use := &stubKeyUse{}
			svc := &ChatService{
				keys:     &stubKeyLookup{key: key, fail: tc.lookupFail},
				settings: stubRequireKey{required: tc.required, err: tc.settingsErr},
				keyUse:   use,
				clock:    time.Now,
			}
			_, err := svc.Authenticate(context.Background(), tc.presented)
			if tc.wantRefused && err == nil {
				t.Fatal("Authenticate() admitted a refused key")
			}
			if !tc.wantRefused && err != nil {
				t.Fatalf("Authenticate() error = %v", err)
			}
			if len(use.ids) != tc.wantUses {
				t.Fatalf("recorded %d uses, want %d", len(use.ids), tc.wantUses)
			}
			if tc.wantUses == 1 && use.ids[0] != key.ID() {
				t.Fatalf("recorded key %q, want %q", use.ids[0], key.ID())
			}
		})
	}
}

// TestChatService_AuthenticateSurvivesAFailedKeyUseWrite pins that the counter
// is bookkeeping: a write that fails must not refuse a request the key
// authorised, because the client would then be denied a call it may make.
func TestChatService_AuthenticateSurvivesAFailedKeyUseWrite(t *testing.T) {
	key := domain.NewGatewayKey("panel", "sk-live", "sk-…ive", time.Now().UTC())
	svc := &ChatService{
		keys:     &stubKeyLookup{key: key},
		settings: stubRequireKey{required: true},
		keyUse:   &stubKeyUse{fail: errors.New("the key database is down")},
		clock:    time.Now,
	}
	admitted, err := svc.Authenticate(context.Background(), "sk-live")
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want the key admitted", err)
	}
	if admitted.ID() != key.ID() {
		t.Fatalf("key = %q, want %q", admitted.ID(), key.ID())
	}
}
