// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/auth_flows_test.go
// @for       The login table, the lockout reset, the password change and the session lifecycle.
// @uses      testing, internal/domain.
// @reason    Four flows over one AuthService, driven by the port doubles that live
//
//	in auth_test.go, so the behaviour is pinned without a database or a
//	session store.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestAuthLoginTable(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  string
	}{
		{name: "correct password", password: "correct-123", wantErr: ""},
		{name: "wrong password", password: "wrong-123", wantErr: "UNAUTHORIZED"},
		{name: "empty password", password: "", wantErr: "UNAUTHORIZED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _ := newAuthForTest(t, "correct-123")
			token, err := svc.Login(context.Background(), "127.0.0.1", tc.password)
			if tc.wantErr == "" {
				if err != nil || token == "" {
					t.Fatalf("Login() token=%q err=%v", token, err)
				}
				return
			}
			if err == nil || domain.AsAppError(err).Code != tc.wantErr {
				t.Fatalf("Login() err=%v, want %s", err, tc.wantErr)
			}
		})
	}
}
func TestAuthLockoutAndReset(t *testing.T) {
	svc, _, _, limiter := newAuthForTest(t, "correct-123")
	for i := 0; i < 2; i++ {
		if _, err := svc.Login(context.Background(), "client", "bad"); err == nil {
			t.Fatal("failed login unexpectedly succeeded")
		}
	}
	if _, err := svc.Login(context.Background(), "client", "bad"); domain.AsAppError(err).Code != "RATE_LIMITED" {
		t.Fatalf("third failure error=%v, want RATE_LIMITED", err)
	}
	if _, err := svc.Login(context.Background(), "client", "correct-123"); domain.AsAppError(err).Code != "RATE_LIMITED" {
		t.Fatalf("locked login error=%v, want RATE_LIMITED", err)
	}
	limiter.mu.Lock()
	limiter.locked["client"] = 0
	limiter.mu.Unlock()
	if _, err := svc.Login(context.Background(), "client", "correct-123"); err != nil {
		t.Fatalf("login after lock expiry: %v", err)
	}
	if limiter.resets == 0 {
		t.Fatal("successful login did not reset limiter")
	}
}

// TestAuthChangePasswordRevokesEverySession pins that a password change signs
// out every session the account holds, the one that changed it included: a
// credential whose holder just rotated it must not leave any of its sessions
// alive (R12 of docs/DRAFT/042-CODE-REVIEW-FIXES.md).
func TestAuthChangePasswordRevokesEverySession(t *testing.T) {
	svc, _, sessions, _ := newAuthForTest(t, "correct-123")
	first, err := svc.Login(context.Background(), "client-a", "correct-123")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Login(context.Background(), "client-b", "correct-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangePassword(context.Background(), first, "correct-123", "renewed-456"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	for _, tc := range []struct{ name, token string }{
		{"the session that changed the password", first},
		{"another live session", second},
	} {
		if err := svc.Authenticate(context.Background(), tc.token); !errors.Is(err, domain.ErrSessionInvalid) {
			t.Fatalf("authenticating %s: err=%v, want the session to be invalid", tc.name, err)
		}
	}
	if len(sessions.active) != 0 {
		t.Fatalf("session store retained %d digests, want none", len(sessions.active))
	}
	token, err := svc.Login(context.Background(), "client-a", "renewed-456")
	if err != nil || token == "" {
		t.Fatalf("login with the new password: token=%q err=%v", token, err)
	}
}
func TestAuthSessionLifecycle(t *testing.T) {
	svc, _, sessions, _ := newAuthForTest(t, "correct-123")
	token, err := svc.Login(context.Background(), "client", "correct-123")
	if err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status(context.Background(), token)
	if err != nil || !status.Authenticated || !status.PasswordConfigured || !status.RequireLogin {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if err := svc.Authenticate(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	status, err = svc.Status(context.Background(), token)
	if err != nil || status.Authenticated {
		t.Fatalf("revoked status=%+v err=%v", status, err)
	}
	if len(sessions.active) != 0 {
		t.Fatal("session store retained revoked session")
	}
}
