// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_alias_guard.go
// @for       The alias half of the node delete guard: a node cannot be deleted while a model alias still targets one of its models.
// @uses      internal/domain, context, strings.
// @reason    An alias is the name a client calls and its target is resolved on every request, so deleting the provider under a live alias leaves that name answering failures for traffic the operator never mentioned. The guard reads the alias table and names the alias instead. An alias pointing at a combo is covered by the combo guard, because §7.6 allows one dereference level.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-08
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// AliasLister lists the stored model aliases, which is what makes a node delete
// refuse while an alias still targets one of the node's models. It is satisfied by
// the concrete model catalog repository.
type AliasLister interface {
	Aliases(ctx context.Context) ([]domain.ModelAlias, error)
}

// rejectAliasReference refuses a delete while a stored alias targets the node, in
// either spelling the router accepts (the node's id or its prefix, which a combo
// member is canonicalized through too), so an alias cannot outlive the provider its
// target names and keep answering about a model nobody can route.
func (s *NodeService) rejectAliasReference(ctx context.Context, nodeID, prefix string) error {
	if s.aliases == nil {
		return nil
	}
	aliases, err := s.aliases.Aliases(ctx)
	if err != nil {
		return err
	}
	for _, alias := range aliases {
		member, _, _ := strings.Cut(alias.Target(), "/")
		if member == nodeID || (prefix != "" && member == prefix) {
			return domain.NewConflictError("alias " + alias.Alias() + " still targets this provider")
		}
	}
	return nil
}
