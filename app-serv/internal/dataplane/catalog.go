// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/catalog.go
// @for       The models list the data plane publishes, in the OpenAI list shape.
// @uses      internal/schema, internal/registry, context, sort.
// @reason    SPEC-API-001 §7.15 serves GET /api/v1/models as `{object:"list",
//
//	data:[...]}` over the routable models and combos, and §7.6 makes the
//	disabled set hide a model from the catalog and from routing alike.
//	Building the list from the same registry, the same translator check,
//	and the same disabled set the router reads is what keeps "listed" and
//	"answerable" one property instead of two.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"sort"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// ModelList assembles the OpenAI models list: every model a client could actually
// route, then every combo by name, sorted so two calls agree.
//
// The rules that make a row are the router's own:
//
// A provider the gateway cannot translate contributes nothing, because a model
// string the router would refuse is not a model a client can use.
//
// A provider with no endpoint the router would pick contributes nothing, unless the
// router serves it on a synthesized endpoint. This is the router's candidate
// question, not a health check: an endpoint in a backoff window still counts,
// because the router will pick it again when the window closes. Naming a model the
// selector has no candidate for is the failure draft 021 F8 measured, and a picker
// built on the list offers it.
//
// A model an operator added lists even where the provider declares no catalog,
// which is draft 021 F10: a custom node's model list is what the operator typed,
// because its upstream may refuse to enumerate itself. A row naming a provider the
// registry does not know is skipped rather than invented.
//
// A combo lists either way, because it is addressed by name and the selector walks
// its members per request (SPEC-API-001 §7.15).
func (r *Resolver) ModelList(ctx context.Context) (schema.ModelList, error) {
	disabledPairs, err := r.lookup.DisabledPairs(ctx)
	if err != nil {
		return schema.ModelList{}, err
	}
	disabled := make(map[disabledKey]struct{}, len(disabledPairs))
	for _, ref := range disabledPairs {
		disabled[disabledKey{provider: ref.ProviderID(), model: ref.ModelID()}] = struct{}{}
	}

	customPairs, err := r.lookup.CustomModels(ctx)
	if err != nil {
		return schema.ModelList{}, err
	}

	entries := r.index.All()
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	active, err := r.lookup.ActiveProviders(ctx, ids)
	if err != nil {
		return schema.ModelList{}, err
	}

	listed := make(map[string]schema.ModelObject, len(entries)*4)
	for _, entry := range entries {
		if !r.listable(entry, active) {
			continue
		}
		for _, model := range entry.Models {
			if !model.IsChat() {
				continue
			}
			r.addRow(listed, entry, model.ID, disabled)
		}
	}
	for _, ref := range customPairs {
		entry, found := r.index.Provider(ref.ProviderID())
		if !found {
			continue
		}
		if !r.listable(entry, active) {
			continue
		}
		r.addRow(listed, entry, ref.ModelID(), disabled)
	}

	comboNames, err := r.lookup.ComboNames(ctx)
	if err != nil {
		return schema.ModelList{}, err
	}
	for _, name := range comboNames {
		if _, seen := listed[name]; !seen {
			listed[name] = schema.ModelObject{ID: name, Object: "model", OwnedBy: "combo"}
		}
	}

	list := schema.ModelList{Object: "list", Data: make([]schema.ModelObject, 0, len(listed))}
	for _, row := range listed {
		list.Data = append(list.Data, row)
	}
	sort.SliceStable(list.Data, func(a, b int) bool { return list.Data[a].ID < list.Data[b].ID })
	return list, nil
}

// listable reports whether the router has anywhere to send a request for this
// provider: a candidate endpoint it would pick, or no credential to pick with.
func (r *Resolver) listable(entry registry.Provider, active map[string]bool) bool {
	if !entry.IsChatRoutable() || targetFormat(entry.Transport.Format) == "" {
		return false
	}
	return active[entry.ID] || entry.NeedsNoCredential()
}

// addRow writes one listed model, keyed by the id a client sends so a custom row
// duplicating a declared model cannot list twice.
//
// The id is spelled the way the router accepts and the operator typed it: a custom
// node by the prefix it was created with, a registry provider by its id. Both
// resolve, and the prefix is the only one either side learned from the operator.
func (r *Resolver) addRow(
	into map[string]schema.ModelObject,
	entry registry.Provider,
	modelID string,
	disabled map[disabledKey]struct{},
) {
	// §7.6 keeps the spelling the operator typed in the disabled pair, so a disable
	// written as the node prefix and one written as the node id are the same rule.
	// Testing only the canonical id would list a model the router refuses.
	if r.isDisabled(disabled, entry, modelID) {
		return
	}
	owner := listedOwner(entry)
	id := owner + "/" + modelID
	into[id] = schema.ModelObject{ID: id, Object: "model", OwnedBy: owner}
}

// isDisabled reports whether a disabled pair names this model under any provider
// spelling the entry answers to: the canonical id the registry keys by, plus the
// alias or node prefix an operator writes.
func (r *Resolver) isDisabled(
	disabled map[disabledKey]struct{},
	entry registry.Provider,
	modelID string,
) bool {
	for _, provider := range append([]string{entry.ID, entry.Alias}, entry.Aliases...) {
		if provider == "" {
			continue
		}
		if _, hidden := disabled[disabledKey{provider: provider, model: modelID}]; hidden {
			return true
		}
	}
	return false
}

// listedOwner returns the provider segment a listed id carries: the node's own
// prefix for a node, the registry id for everything else.
func listedOwner(entry registry.Provider) string {
	if entry.Custom && entry.Alias != "" {
		return entry.Alias
	}
	return entry.ID
}

// disabledKey is the (provider, model) pair the disabled set is keyed by, so the
// membership test is one map lookup rather than a scan per model.
type disabledKey struct {
	provider string
	model    string
}
