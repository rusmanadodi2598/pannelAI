// Package repository is the storage boundary app-serv services depend on.
//
// @file      internal/repository/proxy.go
// @for       Storage contract for saved outbound proxy candidates (SPEC-API-001 §7.11).
// @uses      context, internal/domain.
// @reason    AGENTS.md §1.5 keeps services off the driver, and §2.2 makes the
//
//	candidate the aggregate root: its status document is written whole
//	with every update, so the contract exposes the aggregate and never
//	a status column on its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package repository

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ProxyRepository is the storage boundary for proxy candidates (SPEC-API-001
// §7.11).
type ProxyRepository interface {
	// Create persists a new candidate. The pool is small and read whole, so
	// there is no uniqueness rule beyond the id.
	Create(ctx context.Context, proxy domain.Proxy) error

	// List returns every candidate, ordered by label so the panel's table is
	// stable. §7.11 lists the pool without pagination: an operator keeps a
	// handful of candidates, not a page of them.
	List(ctx context.Context) ([]domain.Proxy, error)

	// GetByID loads one candidate. A missing row must yield
	// domain.ErrProxyNotFound.
	GetByID(ctx context.Context, id string) (domain.Proxy, error)

	// Update persists every mutable field, including the status document, as
	// one statement.
	Update(ctx context.Context, proxy domain.Proxy) error

	// Delete removes one candidate. A missing row must yield
	// domain.ErrProxyNotFound.
	Delete(ctx context.Context, id string) error
}
