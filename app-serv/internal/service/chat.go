// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/chat.go
// @for       The chat data plane use case: data-plane authentication, the
//
//	resolve-translate-call pipeline, and usage recording.
//
// @uses      internal/dataplane, internal/domain, internal/schema, context.
// @reason    SPEC-API-001 §4 enforces the gateway key when
//
//	settings.security.require_api_key is true, and §7.15 makes recording
//	one row per served request part of the pipeline. Both are
//	orchestration — not transport, not SQL — so they live here where no
//	net/http import is needed (AGENTS.md §1.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// GatewayKeyLookup resolves a presented data plane credential to the stored key.
// It is a one-method seam because the data plane needs exactly one question
// answered, and the composition root adapts the gateway key repository to it.
type GatewayKeyLookup interface {
	// GetByValueHash loads an active, non-revoked key by its digest. A missing
	// row must yield domain.ErrGatewayKeyNotFound.
	GetByValueHash(ctx context.Context, valueHash string) (domain.GatewayKey, error)
}

// RequireAPIKeyReader reports whether the data plane must authenticate a request.
// It is an interface so the settings reader is a dependency rather than a package
// import, and so this service stays testable without a database.
type RequireAPIKeyReader interface {
	RequireAPIKey(ctx context.Context) (bool, error)
}

// UsageRecorder records one served request's accounting. It is an interface for
// the same reason as the other two: the data plane writes through it and must not
// depend on a repository contract it does not read.
type UsageRecorder interface {
	Record(ctx context.Context, in domain.UsageRecordInput) (domain.UsageRecord, error)
}

// RequestIDReader reads the router's request id from a context, so a usage row and
// the log line for one request share an identifier (SPEC-API-001 §4). It is a
// function value rather than a router import, because the service layer must not
// depend on the HTTP layer (AGENTS.md §1.5).
type RequestIDReader func(ctx context.Context) string

// ChatService implements the chat side of SPEC-API-001 §7.15.
type ChatService struct {
	engine    *dataplane.Engine
	keys      GatewayKeyLookup
	settings  RequireAPIKeyReader
	usage     UsageRecorder
	logs      RequestLogRecorder
	keyUse    KeyUseRecorder
	requestID RequestIDReader
	clock     func() time.Time
}

// ChatServiceDeps holds the collaborators the service needs.
type ChatServiceDeps struct {
	Engine   *dataplane.Engine
	Keys     GatewayKeyLookup
	Settings RequireAPIKeyReader
	Usage    UsageRecorder
	// Logs writes the request-log half of the §7.13 accounting pair. Optional:
	// without one the data plane serves and records usage only.
	Logs RequestLogRecorder
	// KeyUse advances the presenting key's own counters. Optional: without one
	// the data plane serves every request and records no key usage.
	KeyUse    KeyUseRecorder
	RequestID RequestIDReader
}

// NewChatService validates deps and returns a ready service.
//
// The key lookup and the settings reader are required together: authentication is
// enforced only when both are present, because a half-wired pair would either skip
// the check or fail every request for a reason the operator did not configure.
func NewChatService(deps ChatServiceDeps) (*ChatService, error) {
	if deps.Engine == nil {
		return nil, domain.NewValidationError("data plane engine is required")
	}
	if (deps.Keys == nil) != (deps.Settings == nil) {
		return nil, domain.NewValidationError("gateway key lookup and settings reader must be wired together")
	}
	return &ChatService{
		engine: deps.Engine, keys: deps.Keys, settings: deps.Settings,
		usage: deps.Usage, logs: deps.Logs, keyUse: deps.KeyUse,
		requestID: deps.RequestID, clock: time.Now,
	}, nil
}

// Authenticate applies the §4 data plane rule and returns the key that presented
// the credential, or a zero key when authentication is disabled.
//
// The presented secret is hashed rather than compared: the stored value is a
// digest, so the plaintext never has to be recoverable for a request to be
// authorised (SPEC-API-001 §6).
func (s *ChatService) Authenticate(ctx context.Context, presented string) (domain.GatewayKey, error) {
	if s.keys == nil || s.settings == nil {
		return domain.GatewayKey{}, nil
	}
	required, err := s.settings.RequireAPIKey(ctx)
	if err != nil {
		// A settings read that fails is treated as "required", which is the
		// documented default and the safe direction: a data plane that cannot tell
		// whether it must authenticate must not skip authenticating.
		return domain.GatewayKey{}, domain.NewInternalError("authentication could not be checked")
	}
	if !required {
		return domain.GatewayKey{}, nil
	}
	token := strings.TrimSpace(presented)
	if token == "" {
		return domain.GatewayKey{}, domain.NewUnauthorizedError("a gateway key is required")
	}
	key, err := s.keys.GetByValueHash(ctx, domain.HashKey(token))
	if err != nil {
		// The reason is deliberately not forwarded: whether the key is unknown or
		// revoked is not something an unauthenticated caller should learn.
		return domain.GatewayKey{}, domain.NewUnauthorizedError("the gateway key is not valid")
	}
	s.recordKeyUse(ctx, key)
	return key, nil
}

// Relay runs one chat request through the pipeline and records its accounting.
func (s *ChatService) Relay(ctx context.Context, in dataplane.Request, sink dataplane.FrameSink, keyID string) (dataplane.Outcome, error) {
	outcome, err := s.engine.Relay(ctx, in, sink)
	if err != nil {
		s.record(ctx, in, outcome, keyID, dataplane.AsError(err).Code)
		return dataplane.Outcome{}, err
	}
	s.record(ctx, in, outcome, keyID, "")
	return outcome, nil
}

// Models lists the routable models and combos in the OpenAI list shape (§7.15).
func (s *ChatService) Models(ctx context.Context) (schema.ModelList, error) {
	return s.engine.Resolver().ModelList(ctx)
}
