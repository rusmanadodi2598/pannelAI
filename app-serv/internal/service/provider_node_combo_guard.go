// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_combo_guard.go
// @for       The combo half of the node delete guard (draft 028 F3): a node
//
//	cannot be deleted while a stored combo still names it.
//
// @uses      internal/domain, internal/repository, context, strings.
// @reason    A combo member that outlives its provider is attempted on every
//
//	request and answers with a refusal about a model the client never
//	named. The guard reads the combo table, which the node lifecycle
//	file does not otherwise touch, and keeping it here holds
//	provider_node.go inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ComboLister lists the stored combos, which is what makes a node delete refuse
// while a combo still references the node as a member (draft 028 F3): a combo
// whose member loses its provider is attempted on every request and answers
// with a refusal about a model the client never named. It is satisfied by the
// concrete combo repository.
type ComboLister interface {
	List(ctx context.Context, q repository.PageQuery) ([]domain.Combo, int64, error)
}

// maxComboScan bounds the combo listing a delete guard reads, so the check is a
// bounded query (AGENTS.md §1.7) while still covering every combo a real
// deployment holds.
const maxComboScan = 1000

// rejectComboReference refuses a delete while a stored combo still names the
// node as a member, in either spelling the router accepts (the node's id or
// its prefix, which combo_order.go canonicalizes through the same lookup), so
// a stale member cannot answer with a refusal about a model nobody named.
func (s *NodeService) rejectComboReference(ctx context.Context, nodeID, prefix string) error {
	if s.combos == nil {
		return nil
	}
	combos, _, err := s.combos.List(ctx, repository.PageQuery{Page: 1, PerPage: maxComboScan})
	if err != nil {
		return err
	}
	for _, combo := range combos {
		for _, ref := range combo.Refs() {
			member, _, _ := strings.Cut(ref, "/")
			if member == nodeID || (prefix != "" && member == prefix) {
				return domain.NewConflictError("combo " + combo.Name() + " still references this provider")
			}
		}
	}
	return nil
}
