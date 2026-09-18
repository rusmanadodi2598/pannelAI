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

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// ModelList assembles the OpenAI models list: every routable model as
// `provider/model`, then every combo by name, sorted so two calls agree.
//
// A provider the gateway cannot translate contributes nothing, because a model
// string the router would refuse is not a model a client can use. A model a
// provider does not declare is not invented: a passthrough provider enumerates no
// catalog, so listing one id for it would be a guess the router would resolve by
// its own rule.
func (r *Resolver) ModelList(ctx context.Context) (schema.ModelList, error) {
	list := schema.ModelList{Object: "list", Data: make([]schema.ModelObject, 0, 64)}

	disabledPairs, err := r.lookup.DisabledPairs(ctx)
	if err != nil {
		return schema.ModelList{}, err
	}
	disabled := make(map[disabledKey]struct{}, len(disabledPairs))
	for _, ref := range disabledPairs {
		disabled[disabledKey{provider: ref.ProviderID(), model: ref.ModelID()}] = struct{}{}
	}

	for _, entry := range r.index.All() {
		if !entry.IsChatRoutable() || targetFormat(entry.Transport.Format) == "" {
			continue
		}
		for _, model := range entry.Models {
			if !model.IsChat() {
				continue
			}
			if _, hidden := disabled[disabledKey{provider: entry.ID, model: model.ID}]; hidden {
				continue
			}
			list.Data = append(list.Data, schema.ModelObject{
				ID:      entry.ID + "/" + model.ID,
				Object:  "model",
				OwnedBy: entry.ID,
			})
		}
	}

	comboNames, err := r.lookup.ComboNames(ctx)
	if err != nil {
		return schema.ModelList{}, err
	}
	for _, name := range comboNames {
		list.Data = append(list.Data, schema.ModelObject{ID: name, Object: "model", OwnedBy: "combo"})
	}

	sort.SliceStable(list.Data, func(a, b int) bool { return list.Data[a].ID < list.Data[b].ID })
	return list, nil
}

// disabledKey is the (provider, model) pair the disabled set is keyed by, so the
// membership test is one map lookup rather than a scan per model.
type disabledKey struct {
	provider string
	model    string
}
