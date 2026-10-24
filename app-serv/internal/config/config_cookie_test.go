// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_cookie_test.go
// @for       The session cookie's deployment policy at parse and validation time.
// @uses      testing, standard library only.
// @reason    Secure is the default in every profile, so the only way to lose it is the explicit development opt-out; production refusing that value has to be proven at boot, not discovered in a browser.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-24
package config

import "testing"

func TestLoad_SessionCookieInsecure(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		want    bool
		wantErr bool
	}{
		{"default secure", nil, false, false},
		{"local opt-out", map[string]string{"SESSION_COOKIE_INSECURE": "true"}, true, false},
		{"production keeps secure", map[string]string{"APP_ENV": "production"}, false, false},
		{"production refuses opt-out", map[string]string{"APP_ENV": "production", "SESSION_COOKIE_INSECURE": "true"}, false, true},
		{"malformed", map[string]string{"SESSION_COOKIE_INSECURE": "maybe"}, false, true},
		{"empty", map[string]string{"SESSION_COOKIE_INSECURE": ""}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			for k, v := range tc.env {
				env[k] = v
			}
			setEnv(t, env)
			cfg, err := Load()
			if tc.wantErr {
				if err == nil {
					t.Fatal("Load() = nil error, want rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.SessionCookieInsecure != tc.want {
				t.Fatalf("SessionCookieInsecure = %v, want %v", cfg.SessionCookieInsecure, tc.want)
			}
		})
	}
}
