// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_canonical.go
// @for       The one rule that decides whether a model reference names a model
//
//	the chat plane can serve, shared by every write path that accepts one.
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
//	The judgement is the router's own, not a second opinion about it: a
//	reference is servable when `ResolveParts` would serve it, which is why
//	the passthrough and shadowing rules below are stated in the router's
//	terms rather than read off the catalog row.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// referenceView is one catalog read plus the provider names it was built with,
// so a caller validating many references resolves the index once (AGENTS.md
// §1.7) instead of once per row or per reference.
//
// names maps every spelling a provider answers to (its id, alias, and extra
// aliases) onto the provider entry. Building it from All() rather than calling
// Provider() per row matters in production: the composition root's index adapter
// rebuilds the node overlay on every Provider() call, so a per-row lookup turns
// one panel request into one node-list query per catalog row.
type referenceView struct {
	lookups map[string]domain.CatalogModel
	names   map[string]registry.Provider
}

// newReferenceView reads the catalog once and indexes the provider names once.
func newReferenceView(catalog *ModelCatalogService, ctx context.Context) (referenceView, error) {
	lookups, err := catalog.lookups(ctx)
	if err != nil {
		return referenceView{}, err
	}
	return referenceView{lookups: lookups, names: providerNames(catalog.index)}, nil
}

// providerNames indexes every spelling each provider answers to.
//
// The alias wins a name it shares with another provider's id, because that is
// the router's own rule (`registry/load.go`: alias-first resolution, so
// `mmf/...` reaches the alias owner, not the hidden entry that carries that id).
// A write path that preferred the id would accept a reference the router
// resolves to a different provider, which is the drift this file exists to
// prevent.
func providerNames(index CatalogIndex) map[string]registry.Provider {
	entries := index.All()
	names := make(map[string]registry.Provider, len(entries)*2)
	for _, entry := range entries {
		names[entry.ID] = entry
	}
	for _, entry := range entries {
		for _, alias := range providerAliasNames(entry) {
			names[alias] = entry
		}
	}
	return names
}

// providerAliasNames returns the alias spellings a provider answers to, in the
// order the registry declares them.
func providerAliasNames(entry registry.Provider) []string {
	aliases := make([]string, 0, 1+len(entry.Aliases))
	if alias := strings.TrimSpace(entry.Alias); alias != "" && alias != entry.ID {
		aliases = append(aliases, alias)
	}
	for _, alias := range entry.Aliases {
		if alias = strings.TrimSpace(alias); alias != "" && alias != entry.ID {
			aliases = append(aliases, alias)
		}
	}
	return aliases
}

// providerFor resolves a reference's first segment the way the router does: the
// alias table first, then the id.
func (v referenceView) providerFor(name string) (registry.Provider, bool) {
	entry, ok := v.names[strings.TrimSpace(name)]
	return entry, ok
}

// catalogKeyFor resolves a reference to the catalog key that names it, if the
// catalog holds a row for it. An undeclared id on a passthrough provider has no
// row and is answered by servability instead, which is why a miss here is not
// yet a refusal.
func (v referenceView) catalogKeyFor(ref domain.ModelRef) (string, bool) {
	if ref.IsZero() {
		return "", false
	}
	exact := ref.String()
	if _, ok := v.lookups[exact]; ok {
		return exact, true
	}
	entry, ok := v.providerFor(ref.ProviderID())
	if !ok {
		return "", false
	}
	canonical := entry.ID + "/" + ref.ModelID()
	if _, ok := v.lookups[canonical]; ok {
		return canonical, true
	}
	return "", false
}

// resolvesIn reports whether a reference names something the data plane could
// route: a catalog row, or an undeclared id on a provider that passes model ids
// through. It is the existence half of the rule, and the router's own test —
// `ResolveParts` answers a declared model, then a passthrough provider's
// arbitrary id, then refuses everything else.
func (v referenceView) resolvesIn(ref domain.ModelRef) bool {
	if _, ok := v.catalogKeyFor(ref); ok {
		return true
	}
	entry, ok := v.providerFor(ref.ProviderID())
	if !ok {
		return false
	}
	// A custom node is a passthrough provider by construction (`Custom` entries
	// carry the operator's model list, and an id beyond it still routes), so it
	// needs no separate branch here.
	return entry.PassthroughModels || entry.Custom
}

// chatServable answers whether a catalog reference can be served by the chat
// data plane, and the sentence naming why not when it cannot. It is the
// write-time half of the property the §7.15 list already holds: a model listed
// there is answerable, and a reference the router would refuse must not save.
//
// Three refusals exist, and each is the router's own answer rather than a rule
// invented here:
//
//   - the first segment names no provider, so every request to it answers
//     MODEL_NOT_FOUND;
//   - the provider's wire format has no translator, so every request answers
//     PROVIDER_NOT_ROUTABLE;
//   - the provider declares models and does not pass ids through, and the id is
//     not among them, so the request answers MODEL_NOT_FOUND;
//   - the row is a media model (`kind` image/tts/stt/embedding/...), which the
//     chat selector never reaches.
//
// A declared chat model, an undeclared id on a passthrough provider, and a
// custom node's id are all servable: the rule must not widen into refusing what
// routing accepts.
func (v referenceView) chatServable(ref domain.ModelRef) error {
	entry, ok := v.providerFor(ref.ProviderID())
	if !ok {
		return domain.NewValidationError("provider " + ref.ProviderID() + " is not in the registry")
	}
	if routable, why := registryChatRoutable(entry); !routable {
		return domain.NewValidationError(why)
	}
	key, declared := v.catalogKeyFor(ref)
	if !declared {
		if !entry.PassthroughModels && !entry.Custom {
			return domain.NewValidationError("provider " + entry.ID +
				" does not pass model ids through, and model " + ref.ModelID() + " is not declared")
		}
		return nil
	}
	row := v.lookups[key]
	if kind := strings.TrimSpace(row.Kind()); kind != "" && kind != "llm" && kind != "chat" {
		return domain.NewValidationError("model " + row.Ref().String() +
			" is a media model (" + kind + "), not a chat model")
	}
	return nil
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
