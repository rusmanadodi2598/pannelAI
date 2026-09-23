// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_canonical.go
// @for       The one rule that decides whether a model reference names a
//
//	catalog row, shared by every write path that accepts one.
//
// @uses      internal/domain, internal/registry, strings.
// @reason    Draft 024 §3.2 measured the drift this file closes: the router
//
//	resolves three spellings of the first segment (provider id, registry
//	alias, node prefix) while the write paths read one, so a combo member
//	spelled `cc/claude-...` or `oczen/big-pickle` was refused by a list the
//	same gateway routes. Two answers to "does this model exist" is how a
//	write and a request come to disagree, so the question is asked here
//	once and every caller asks it here.
//
//	The reference never validates a combo reference at all — its picker
//	offers only active connections and it trusts the operator (draft 024
//	§1). This port validates at write time instead, which is a decision
//	§7.7 already records; what this file fixes is that the validation
//	disagreed with the router about what a name is.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// catalogKeyFor resolves a reference to the catalog key that names it.
//
// The exact key is tried first because that is the operator's own spelling and
// the common case. When it misses, the first segment is resolved through the
// index — the same lookup the router performs — and the key is rebuilt under
// the provider's canonical id. A reference naming an unknown namespace, or one
// whose model the canonical provider does not hold, answers false.
//
// The map is a parameter rather than fetched here so a caller validating many
// references reads the catalog once (AGENTS.md §1.7).
func catalogKeyFor(lookups map[string]domain.CatalogModel, index CatalogIndex, ref domain.ModelRef) (string, bool) {
	if ref.IsZero() {
		return "", false
	}
	exact := ref.String()
	if _, ok := lookups[exact]; ok {
		return exact, true
	}
	entry, ok := index.Provider(ref.ProviderID())
	if !ok {
		return "", false
	}
	canonical := entry.ID + "/" + ref.ModelID()
	if _, ok := lookups[canonical]; ok {
		return canonical, true
	}
	return "", false
}

// providerNameSet returns every spelling the provider a filter names answers
// to: its id, its alias, and its extra aliases. A node's prefix is its alias,
// which is what makes `provider_id=oczen` find rows stored under the node's id.
//
// An unknown name yields the name itself, which matches no row: a filter for a
// provider the registry does not hold answers an empty page rather than
// accidentally widening to everything.
func providerNameSet(index CatalogIndex, name string) map[string]struct{} {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	entry, ok := index.Provider(trimmed)
	if !ok {
		return map[string]struct{}{trimmed: {}}
	}
	set := make(map[string]struct{}, 2+len(entry.Aliases))
	set[entry.ID] = struct{}{}
	if alias := strings.TrimSpace(entry.Alias); alias != "" {
		set[alias] = struct{}{}
	}
	for _, alias := range entry.Aliases {
		if alias = strings.TrimSpace(alias); alias != "" {
			set[alias] = struct{}{}
		}
	}
	return set
}

// resolvesIn reports whether a reference names a row of one catalog read.
func resolvesIn(lookups map[string]domain.CatalogModel, index CatalogIndex, ref domain.ModelRef) bool {
	_, ok := catalogKeyFor(lookups, index, ref)
	return ok
}

// registryChatRoutable reports whether a provider can serve a chat request
// today, and the sentence naming why not when it cannot.
//
// It exists so a write path and the router answer the same question with the
// same words: the router refuses a provider whose wire format it does not
// translate (SPEC-API-001 §8, PROVIDER_NOT_ROUTABLE), and a combo member that
// can never be served must not save. The reason is returned rather than a bare
// bool because the operator needs to know whether to add a connector, pick
// another provider, or pick another model.
func registryChatRoutable(entry registry.Provider) (bool, string) {
	if entry.IsChatRoutable() {
		return true, ""
	}
	return false, "provider " + entry.ID + " speaks " + entry.Transport.Format +
		", which the gateway does not translate yet"
}

// chatServable answers whether a catalog reference can be served by the chat
// data plane, and the sentence naming why not when it cannot. It is the
// write-time half of the property the §7.15 list already holds: a model listed
// there is answerable, and a reference the router would refuse must not save.
//
// Two refusals exist:
//
//   - the provider's wire format has no translator, so every request to it
//     answers PROVIDER_NOT_ROUTABLE (the sentence registryChatRoutable builds);
//   - the row is a media model (`kind` image/tts/stt/embedding/...), which the
//     chat selector never reaches — the media plane serves it through its own
//     routes, and a combo member is a chat member by definition.
//
// A declared chat model, a passthrough provider's undeclared id, and a custom
// row on a chat provider are all servable: the rule must not widen into
// refusing what routing accepts (the benign control pins this).
//
// lookups and index arrive as parameters so a caller validating many
// references reads the catalog once (AGENTS.md §1.7).
func chatServable(lookups map[string]domain.CatalogModel, index CatalogIndex, ref domain.ModelRef) error {
	key, ok := catalogKeyFor(lookups, index, ref)
	if !ok {
		// An existence failure is not a routability failure: the caller reports
		// "does not resolve" for this one, which names the actual problem.
		return nil
	}
	row := lookups[key]
	entry, found := index.Provider(row.ProviderID())
	if !found {
		// A catalog row whose provider the index no longer holds predates this
		// call; the existence check above already decided it exists, so the
		// provider check is the honest next question and it answers itself in
		// the router's own words.
		return domain.NewValidationError("provider " + row.ProviderID() + " is no longer in the registry")
	}
	if ok, why := registryChatRoutable(entry); !ok {
		return domain.NewValidationError(why)
	}
	if kind := strings.TrimSpace(row.Kind()); kind != "" && kind != "llm" && kind != "chat" {
		return domain.NewValidationError("model " + row.Ref().String() +
			" is a media model (" + kind + "), not a chat model")
	}
	return nil
}
