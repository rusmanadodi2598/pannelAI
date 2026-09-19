// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/log_retention_fakes_test.go
// @for       The in-memory doubles the retention worker tests drive: a purger
//
//	that can block, fail, or panic, and the settings and log
//	repositories behind the canonical purge.
//
// @uses      testing, context, encoding/json, sync, time, internal/domain,
//
//	internal/repository.
//
// @reason    The worker's rules (retry state, panic boundary, single-owner
//
//	cycles) and the LogService's dynamic cutoff are exercised
//	against behaviour the doubles define once, so every test reads
//	the same contract instead of restating its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

type retentionSettingsRepo struct {
	mu            sync.Mutex
	retentionDays int
}

func (r *retentionSettingsRepo) Load(context.Context) (map[domain.SettingsKey]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, err := json.Marshal(domain.LoggingSettings{RetentionDays: r.retentionDays})
	if err != nil {
		return nil, err
	}
	return map[domain.SettingsKey]string{domain.SettingsKeyLogging: string(value)}, nil
}

func (*retentionSettingsRepo) Save(context.Context, domain.SettingsKey, string) error { return nil }

type retentionLogRepo struct {
	mu      sync.Mutex
	deleted int64
	cutoffs []time.Time
}

func (*retentionLogRepo) Insert(context.Context, domain.RequestLog) error { return nil }
func (*retentionLogRepo) List(context.Context, domain.LogFilter, repository.PageQuery) ([]domain.RequestLog, int64, error) {
	return nil, 0, nil
}
func (*retentionLogRepo) GetByRequestID(context.Context, string) (domain.RequestLog, error) {
	return domain.RequestLog{}, domain.ErrRequestLogNotFound
}
func (r *retentionLogRepo) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cutoffs = append(r.cutoffs, cutoff)
	return r.deleted, nil
}

func retentionPolicy() LogRetentionPolicy {
	return LogRetentionPolicy{Interval: time.Millisecond, MaxAttempts: 3, Timeout: time.Second}
}

// retentionPurger records calls and can block, fail, or panic on demand.
type retentionPurger struct {
	mu      sync.Mutex
	calls   int
	deleted int64
	err     error
	panicOn bool
	started chan struct{}
	release chan struct{}
}

func (p *retentionPurger) Purge(ctx context.Context) (int64, error) {
	p.mu.Lock()
	p.calls++
	if p.started != nil {
		select {
		case <-p.started:
		default:
			close(p.started)
		}
	}
	panicOn, err, deleted, release := p.panicOn, p.err, p.deleted, p.release
	p.mu.Unlock()
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	if panicOn {
		panic("purger failed catastrophically")
	}
	return deleted, err
}

func (p *retentionPurger) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}
