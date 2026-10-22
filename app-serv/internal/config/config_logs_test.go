// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_logs_test.go
// @for       The request-log row cap that bounds the table between retention runs.
// @uses      testing.
// @reason    The two-day purge can leave a burst unbounded, so the cap and its lower bound are pinned at boot rather than discovered after the disk fills.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-22
package config

import "testing"

func TestLoad_RequestLogMaxRows(t *testing.T) {
	setEnv(t, validEnv())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RequestLogMaxRows != 20000 {
		t.Fatalf("RequestLogMaxRows = %d, want 20000", cfg.RequestLogMaxRows)
	}

	env := validEnv()
	env["REQUEST_LOG_MAX_ROWS"] = "500"
	setEnv(t, env)
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() override error = %v", err)
	}
	if cfg.RequestLogMaxRows != 500 {
		t.Fatalf("RequestLogMaxRows = %d, want 500", cfg.RequestLogMaxRows)
	}

	env["REQUEST_LOG_MAX_ROWS"] = "0"
	setEnv(t, env)
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted REQUEST_LOG_MAX_ROWS=0")
	}
}

func TestLoad_UsageRetentionDays(t *testing.T) {
	setEnv(t, validEnv())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.UsageRetentionDays != 90 {
		t.Fatalf("UsageRetentionDays = %d, want 90", cfg.UsageRetentionDays)
	}

	env := validEnv()
	env["USAGE_RETENTION_DAYS"] = "30"
	setEnv(t, env)
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() override error = %v", err)
	}
	if cfg.UsageRetentionDays != 30 {
		t.Fatalf("UsageRetentionDays = %d, want 30", cfg.UsageRetentionDays)
	}

	env["USAGE_RETENTION_DAYS"] = "0"
	setEnv(t, env)
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted USAGE_RETENTION_DAYS=0")
	}
}
