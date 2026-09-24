// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_active.go
// @for       The active-provider predicate of the catalog read: which providers
//
//	the router can actually serve a request to right now.
//
// @uses      internal/domain, context.
// @reason    Draft 025 measured the panel's picker offering 586 of 587 rows
//
//	that answer NO_PROVIDER_AVAILABLE on the first request, because
//	the catalog lists every model the registry declares while the
//	router only reaches providers that hold an active endpoint. The
//	reference answers this in the client (its picker filters
//	activeProviders); this port moves the same question to the server
//	so one parameter, `?active=true`, lets every caller ask it. The
//	predicate is the router's own population, stated in its terms: the
//	candidates query (selection.go) selects endpoints with status
//	active under the provider's canonical id, so one status-active
//	endpoint makes the provider active here, a rate-limited active
//	endpoint keeps it active (the runtime skip is a moment, not a
//	configuration), and disabled or errored endpoints do not. The
//	roll-up is read once per request with every candidate id in that
//	one call, because the same seam is already one query in
//	PostgreSQL and a per-provider loop would turn one read into many.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ActiveProviderSet answers which of the given providers hold at least one
// endpoint the router would still pick as a candidate. It is a seam of its own
// rather than the endpoint roll-up because the roll-up cannot express this set:
// `EndpointStatusCounts.Add` folds a rate-limited endpoint out of Active, and
// health tracking moves an endpoint to error without clearing its backoff
// window, so a stale timestamp would count a dead account as pickable. The
// question "which providers have a candidate" is one predicate over one table
// and is answered as such — in one statement, for every id in the call.
type ActiveProviderSet interface {
	ActiveProviders(ctx context.Context, providerIDs []string) (map[string]bool, error)
}

// activeProviders answers which of the named providers hold a candidate
// endpoint, in one read.
//
// The input is the candidate list the caller already narrowed — every distinct
// provider id among the rows that matched the other filters — so a request that
// asked for one provider measures that provider alone, and a request that asked
// for nothing measures the catalog's own population. Ids are the canonical ids
// the rows carry, which is the same spelling the endpoint table stores and the
// router's candidates query selects by, so no name resolution happens here.
//
// The set is the router's own candidates predicate (`status = 'active'`), which
// is deliberately not the same as "healthy this second": an active endpoint in a
// backoff window is a candidate the router will serve again when the window
// closes, while a disabled or errored endpoint is a configuration an operator
// has to change. A provider whose every endpoint is the former stays offered;
// one whose every endpoint is the latter disappears, which is what makes the
// parameter worth asking for.
//
// A missing seam is a refusal, not an empty answer: a deployment that wired no
// set reader can answer the whole catalog but cannot answer "which providers
// are active", and returning the unfiltered catalog under a parameter that
// promised the opposite would tell the caller every model is usable when
// nothing is.
func (s *ModelCatalogService) activeProviders(ctx context.Context, providerIDs []string) (map[string]bool, error) {
	if s.active == nil {
		return nil, domain.NewInternalError(
			"the active filter requires the endpoint candidate reader, which this deployment does not wire")
	}
	if len(providerIDs) == 0 {
		return map[string]bool{}, nil
	}
	return s.active.ActiveProviders(ctx, providerIDs)
}

// distinctProviderIDs returns the distinct canonical ids of the given rows, in
// first-seen order, so the one candidate read carries each provider once.
func distinctProviderIDs(models []domain.CatalogModel) []string {
	seen := make(map[string]struct{}, len(models))
	ids := make([]string, 0, len(models))
	for _, model := range models {
		id := model.ProviderID()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
