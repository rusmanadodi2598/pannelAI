// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/model_lookup.go
// @for       The catalog read path resolution uses, adapted onto the combo and
//
//	model-catalog repositories.
//
// @uses      internal/domain, internal/repository, context.
// @reason    SPEC-API-001 §7.15 resolves a model string through combos, then
//
//	aliases, then provider/model. Each of those reads is already owned by
//	another boundary, so this file is the seam between them: it satisfies
//	ModelLookup without the resolver depending on four repository
//	contracts, and it is the only place that knows a disabled set is
//	addressable by (provider, model).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ActiveEndpointReader answers which providers hold an endpoint the router would still
// pick. It is declared here rather than reused from internal/service because service
// already imports this package, so the reverse import is a cycle.
type ActiveEndpointReader interface {
	ActiveProviders(ctx context.Context, providerIDs []string) (map[string]bool, error)
}

// CatalogLookup reads the combo, alias, and disabled sets for resolution.
type CatalogLookup struct {
	combos  repository.ComboRepository
	catalog repository.ModelCatalogRepository
	active  ActiveEndpointReader
}

// NewCatalogLookup binds the lookup to the three catalog boundaries. All are
// required: a nil combo repository would silently make every combo name fall
// through to the alias path, and a nil candidate reader would answer "nothing is
// active" for a deployment that simply forgot to wire one. Both are routing
// differences, not degradations.
func NewCatalogLookup(
	combos repository.ComboRepository,
	catalog repository.ModelCatalogRepository,
	active ActiveEndpointReader,
) (*CatalogLookup, error) {
	if combos == nil {
		return nil, domain.NewValidationError("combo repository is required")
	}
	if catalog == nil {
		return nil, domain.NewValidationError("model catalog repository is required")
	}
	if active == nil {
		return nil, domain.NewValidationError("active endpoint reader is required")
	}
	return &CatalogLookup{combos: combos, catalog: catalog, active: active}, nil
}

// Combo returns a combo by name, and whether the name addresses one. The
// aggregate is returned whole because the fusion strategy is executed from it.
//
// A missing combo is "not found", not an error: a model string that is not a
// combo is the ordinary case, and the resolver moves on to the alias path.
func (l *CatalogLookup) Combo(ctx context.Context, name string) (domain.Combo, bool, error) {
	combo, err := l.combos.GetByName(ctx, name)
	if err != nil {
		if domain.AsAppError(err).Code == "NOT_FOUND" {
			return domain.Combo{}, false, nil
		}
		return domain.Combo{}, false, err
	}
	return combo, true, nil
}

// Alias returns an alias's target, and whether the alias exists.
func (l *CatalogLookup) Alias(ctx context.Context, name string) (string, bool, error) {
	aliases, err := l.catalog.Aliases(ctx)
	if err != nil {
		return "", false, err
	}
	for _, alias := range aliases {
		if alias.Alias() == name {
			return alias.Target(), true, nil
		}
	}
	return "", false, nil
}

// Disabled reports whether a model is hidden from routing (§7.6).
//
// The stored set is a small pair list read whole, so this is a membership test
// rather than a query per model; the listing path asks for the whole set once
// instead of calling this per row.
func (l *CatalogLookup) Disabled(ctx context.Context, providerID, modelID string) (bool, error) {
	disabled, err := l.catalog.Disabled(ctx)
	if err != nil {
		return false, err
	}
	for _, ref := range disabled {
		if ref.ProviderID() == providerID && ref.ModelID() == modelID {
			return true, nil
		}
	}
	return false, nil
}

// DisabledPairs returns the whole disabled set, which is what the models list
// filters with: loading it once per request is cheaper than asking about every
// model in the catalog.
func (l *CatalogLookup) DisabledPairs(ctx context.Context) ([]domain.ModelRef, error) {
	return l.catalog.Disabled(ctx)
}

// ComboNames returns every combo name, because a combo is addressed by name as a
// model string (§7.15).
func (l *CatalogLookup) ComboNames(ctx context.Context) ([]string, error) {
	return l.combos.Names(ctx)
}

// CustomModels returns the models an operator added, as provider/model pairs.
//
// The pair is what the listing needs rather than the whole row: a custom model is
// listable by the fact that it is stored, and the row's display name and
// capabilities belong to the management catalog that shows them.
func (l *CatalogLookup) CustomModels(ctx context.Context) ([]domain.ModelRef, error) {
	rows, err := l.catalog.Custom(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]domain.ModelRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, row.Ref())
	}
	return refs, nil
}

// ActiveProviders reports which of the named providers hold an endpoint the router
// would still pick, in the one read the candidate question is answered by.
func (l *CatalogLookup) ActiveProviders(ctx context.Context, providerIDs []string) (map[string]bool, error) {
	return l.active.ActiveProviders(ctx, providerIDs)
}
