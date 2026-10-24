// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_transport_test.go
// @for       The transport policy at boot: verified PostgreSQL TLS outside development and the Redis TLS switch.
// @uses      testing, standard library only.
// @reason    An unset or non-verifying sslmode and a plaintext Redis connection are silent downgrades, so the profiles that must refuse them are pinned here rather than discovered in a deployment.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-24
package config

import "testing"

func TestLoad_PostgresSSLMode(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		dsn     string
		wantErr bool
	}{
		{"development may disable", "development", "postgres://user:pass@localhost:5432/db?sslmode=disable", false},
		{"development may omit", "development", "postgres://user:pass@localhost:5432/db", false},
		{"staging refuses disable", "staging", "postgres://user:pass@db:5432/app?sslmode=disable", true},
		{"staging refuses an omitted mode", "staging", "postgres://user:pass@db:5432/app", true},
		{"staging refuses require, which encrypts without verifying", "staging", "postgres://user:pass@db:5432/app?sslmode=require", true},
		{"production accepts verify-ca", "production", "postgres://user:pass@db:5432/app?sslmode=verify-ca", false},
		{"production accepts verify-full in the key/value form", "production", "host=db user=app sslmode=verify-full", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			env["APP_ENV"] = tc.env
			env["POSTGRES_DSN"] = tc.dsn
			setEnv(t, env)
			if _, err := Load(); (err != nil) != tc.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestLoad_RedisTLS(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantTLS bool
		wantCA  string
		wantErr bool
	}{
		{"default is plaintext", nil, false, "", false},
		{"enabled uses system roots", map[string]string{"REDIS_TLS_ENABLED": "true"}, true, "", false},
		{"enabled with a bundle", map[string]string{"REDIS_TLS_ENABLED": "true", "REDIS_TLS_CA_FILE": "/etc/ssl/redis-ca.pem"}, true, "/etc/ssl/redis-ca.pem", false},
		{"a bundle without TLS is inert and refused", map[string]string{"REDIS_TLS_CA_FILE": "/etc/ssl/redis-ca.pem"}, false, "", true},
		{"malformed switch", map[string]string{"REDIS_TLS_ENABLED": "sometimes"}, false, "", true},
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
			if cfg.RedisTLSEnabled != tc.wantTLS || cfg.RedisTLSCAFile != tc.wantCA {
				t.Fatalf("Redis TLS = %v/%q, want %v/%q", cfg.RedisTLSEnabled, cfg.RedisTLSCAFile, tc.wantTLS, tc.wantCA)
			}
		})
	}
}
