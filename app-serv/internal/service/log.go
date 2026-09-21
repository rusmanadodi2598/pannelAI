// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/log.go
// @for       Request-log read, capture-aware write, purge, and the console
//
//	ring buffer the management route exposes.
//
// @uses      internal/domain, internal/repository, context, time.
// @reason    SPEC-API-001 §7.13 makes body capture and the console buffer bound
//
//	settings-driven, and the purge retention-driven. Deciding that here
//	means the repository only ever stores what the settings allow, and
//	the settings themselves are read through the settings service
//	rather than by a second reader of the same table.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ConsoleBuffer is the bounded ring the console route reads and the gateway
// writes to. It is an interface here so the service layer depends on the
// behavior rather than on Redis (AGENTS.md §1.5).
type ConsoleBuffer interface {
	// Append adds one line, evicting the oldest when the ring is full.
	Append(ctx context.Context, line string, maxRecords int) error
	// Lines returns the buffered lines oldest first, at most maxRecords.
	Lines(ctx context.Context, maxRecords int) ([]string, error)
	// Clear empties the buffer.
	Clear(ctx context.Context) error
}

// LogService implements SPEC-API-001 §7.13.
type LogService struct {
	logs     repository.RequestLogRepository
	settings *SettingsService
	console  ConsoleBuffer
	clock    func() time.Time
}

// LogServiceDeps holds the collaborators the service needs.
type LogServiceDeps struct {
	Logs     repository.RequestLogRepository
	Settings *SettingsService
	Console  ConsoleBuffer
}

// NewLogService validates deps and returns a ready service. The console buffer
// is optional: a deployment without Redis keeps the durable log routes working
// and reports an empty console rather than failing to start.
func NewLogService(deps LogServiceDeps) (*LogService, error) {
	if deps.Logs == nil {
		return nil, domain.NewValidationError("request log repository is required")
	}
	if deps.Settings == nil {
		return nil, domain.NewValidationError("settings service is required")
	}
	return &LogService{logs: deps.Logs, settings: deps.Settings, console: deps.Console, clock: time.Now}, nil
}

// Record stores one request log, applying the capture setting before the write.
//
// The bodies are dropped or truncated by the domain rule, so the repository
// never sees a payload the settings forbid storing. This is the only place the
// capture decision is made, which is what keeps a second writer from storing
// bodies capture is supposed to exclude.
func (s *LogService) Record(ctx context.Context, in domain.RequestLogInput) (domain.RequestLog, error) {
	settings, err := s.settings.Settings(ctx)
	if err != nil {
		return domain.RequestLog{}, err
	}
	in.RequestBody, in.ResponseBody = domain.CaptureBodies(
		in.RequestBody, in.ResponseBody,
		settings.Logging.RequestCaptureEnabled, settings.Logging.CaptureBodyMaxBytes)

	entry, err := domain.NewRequestLog(in, s.clock())
	if err != nil {
		return domain.RequestLog{}, err
	}
	if err := s.logs.Insert(ctx, entry); err != nil {
		return domain.RequestLog{}, fmt.Errorf("storing request log: %w", err)
	}
	return entry, nil
}

// Requests returns one page of log rows with the total the meta block needs.
// The filter is validated here as well as at the wire boundary, so a non-HTTP
// caller cannot hand the repository a status outside the closed set and get a
// silently empty page (draft 010 F2/F9).
func (s *LogService) Requests(ctx context.Context, filter domain.LogFilter, page, perPage int) ([]domain.RequestLog, int64, error) {
	if err := filter.Validate(); err != nil {
		return nil, 0, err
	}
	return s.logs.List(ctx, filter, repository.PageQuery{Page: page, PerPage: perPage})
}

// Request returns one log row with its captured bodies, alongside the capture
// setting in force so the caller can say "capture is off" rather than rendering
// an empty body area (SPEC-UI §6.11).
func (s *LogService) Request(ctx context.Context, requestID string) (domain.RequestLog, bool, int, error) {
	settings, err := s.settings.Settings(ctx)
	if err != nil {
		return domain.RequestLog{}, false, 0, err
	}
	entry, err := s.logs.GetByRequestID(ctx, requestID)
	if err != nil {
		return domain.RequestLog{}, false, 0, err
	}
	return entry, settings.Logging.RequestCaptureEnabled, settings.Logging.CaptureBodyMaxBytes, nil
}

// Purge deletes rows older than the retention setting and reports how many were
// removed. Retention is read from settings rather than passed in, so the purge
// route and the retention worker cannot use two different windows.
//
// The cutoff is exclusive, so a row written exactly at the boundary survives:
// the purge removes what is strictly older than the window.
func (s *LogService) Purge(ctx context.Context) (int64, error) {
	settings, err := s.settings.Settings(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := domain.RetentionCutoff(s.clock(), settings.Logging.RetentionDays)
	deleted, err := s.logs.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purging request logs: %w", err)
	}
	return deleted, nil
}

// Console returns the buffered console lines and the bound in force. A missing
// buffer reports no lines, because a deployment without Redis still has a
// console screen and an empty console is the true answer there.
func (s *LogService) Console(ctx context.Context) ([]string, int, error) {
	settings, err := s.settings.Settings(ctx)
	if err != nil {
		return nil, 0, err
	}
	maxRecords := settings.Logging.ObservabilityMaxRecords
	if s.console == nil {
		return nil, maxRecords, nil
	}
	lines, err := s.console.Lines(ctx, maxRecords)
	if err != nil {
		return nil, 0, fmt.Errorf("reading console buffer: %w", err)
	}
	return lines, maxRecords, nil
}

// ClearConsole empties the buffer server-side, so every panel sees the same
// empty console (SPEC-UI §6.16).
func (s *LogService) ClearConsole(ctx context.Context) error {
	if s.console == nil {
		return nil
	}
	if err := s.console.Clear(ctx); err != nil {
		return fmt.Errorf("clearing console buffer: %w", err)
	}
	return nil
}

// AppendConsole adds one line to the console ring, evicting the oldest when the
// bound is reached. It is exported so the gateway's log handler can mirror a
// line into the buffer the panel reads, and it reads the bound from settings so
// a changed bound takes effect without a restart.
func (s *LogService) AppendConsole(ctx context.Context, line string) error {
	if s.console == nil || line == "" {
		return nil
	}
	settings, err := s.settings.Settings(ctx)
	if err != nil {
		return err
	}
	if err := s.console.Append(ctx, line, settings.Logging.ObservabilityMaxRecords); err != nil {
		return fmt.Errorf("appending to console buffer: %w", err)
	}
	return nil
}
