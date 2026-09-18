// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/model_catalog_types.go
// @for       The catalog rows the panel reads: a custom model, an alias
//
//	mapping, and the merged catalog entry.
//
// @uses      internal/domain (ModelRef, ModelCapabilities, AppError
//
//	constructors), strings, time.
//
// @reason    SPEC-API-001 §7.6 returns registry rows, custom rows, and alias
//
//	rows through one endpoint, so the read shape is declared once here
//	rather than reassembled in each handler. The alias row carries no
//	invariant beyond its shape, because whether a target exists spans
//	two aggregates and is checked by the service.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"time"
)

// CustomModel is a user-added model row. It has no transitions beyond creation
// and removal, so it stays a value object.
type CustomModel struct {
	id           string
	ref          ModelRef
	displayName  string
	capabilities ModelCapabilities
	createdAt    time.Time
}

// NewCustomModel is the only constructor for a custom model.
//
// The id is prefixed here rather than by the caller: CustomModelIDPrefix is this
// package's vocabulary (SPEC-API-001 §4 types every id with a prefix), and a
// constructor that accepted a bare ULID would let one call site produce an id the
// rest of the system cannot classify.
func NewCustomModel(id, providerID, modelID, displayName string, capabilities ModelCapabilities, now time.Time) (CustomModel, error) {
	if strings.TrimSpace(id) == "" {
		return CustomModel{}, NewValidationError("custom model id is required")
	}
	if !strings.HasPrefix(id, CustomModelIDPrefix) {
		id = CustomModelIDPrefix + id
	}
	ref, err := NewModelRef(providerID, modelID)
	if err != nil {
		return CustomModel{}, err
	}
	name, err := validateCatalogText("display_name", displayName)
	if err != nil {
		return CustomModel{}, err
	}
	return CustomModel{
		id:           id,
		ref:          ref,
		displayName:  name,
		capabilities: NewModelCapabilities(capabilities...),
		createdAt:    now.UTC(),
	}, nil
}

// RehydrateCustomModel rebuilds a stored row. For the repository load path only.
func RehydrateCustomModel(id string, ref ModelRef, displayName string, capabilities ModelCapabilities, createdAt time.Time) CustomModel {
	return CustomModel{id: id, ref: ref, displayName: displayName, capabilities: capabilities, createdAt: createdAt}
}

func (m CustomModel) ID() string                      { return m.id }
func (m CustomModel) Ref() ModelRef                   { return m.ref }
func (m CustomModel) ProviderID() string              { return m.ref.ProviderID() }
func (m CustomModel) ModelID() string                 { return m.ref.ModelID() }
func (m CustomModel) DisplayName() string             { return m.displayName }
func (m CustomModel) Capabilities() ModelCapabilities { return m.capabilities }
func (m CustomModel) CreatedAt() time.Time            { return m.createdAt }

// ModelAlias is one alias → target mapping of the replaced set (§7.6). The
// alias is the natural key; the target is a provider/model reference or a combo
// name, and whether it exists is the service's check because it spans two
// aggregates.
type ModelAlias struct {
	alias  string
	target string
}

// NewModelAlias validates one mapping.
func NewModelAlias(alias, target string) (ModelAlias, error) {
	name, err := validateCatalogText("alias", alias)
	if err != nil {
		return ModelAlias{}, err
	}
	if strings.Contains(name, "/") {
		return ModelAlias{}, NewValidationError("alias must not contain a slash")
	}
	destination, err := validateComboRef(target)
	if err != nil {
		return ModelAlias{}, err
	}
	return ModelAlias{alias: name, target: destination}, nil
}

func (a ModelAlias) Alias() string  { return a.alias }
func (a ModelAlias) Target() string { return a.target }

// RehydrateModelAlias rebuilds a stored mapping. For the repository load path
// only: the target was resolved when it was written, and re-checking it on read
// would turn a deleted combo into a failed list.
func RehydrateModelAlias(alias, target string) ModelAlias {
	return ModelAlias{alias: alias, target: target}
}

// CatalogSource marks where a catalog row came from, so the panel can tell a
// registry model from one the operator added.
type CatalogSource string

const (
	CatalogSourceRegistry CatalogSource = "registry"
	CatalogSourceCustom   CatalogSource = "custom"
)

// CatalogModel is one merged catalog row: the read shape of §7.6. It carries no
// invariant of its own, so it is built by the service that merged it rather
// than validated on construction.
type CatalogModel struct {
	ref          ModelRef
	displayName  string
	kind         string
	capabilities ModelCapabilities
	source       CatalogSource
}

// NewCatalogModel assembles one merged row.
func NewCatalogModel(ref ModelRef, displayName, kind string, capabilities ModelCapabilities, source CatalogSource) CatalogModel {
	return CatalogModel{
		ref:          ref,
		displayName:  displayName,
		kind:         kind,
		capabilities: NewModelCapabilities(capabilities...),
		source:       source,
	}
}

func (m CatalogModel) Ref() ModelRef                   { return m.ref }
func (m CatalogModel) ProviderID() string              { return m.ref.ProviderID() }
func (m CatalogModel) ModelID() string                 { return m.ref.ModelID() }
func (m CatalogModel) DisplayName() string             { return m.displayName }
func (m CatalogModel) Kind() string                    { return m.kind }
func (m CatalogModel) Capabilities() ModelCapabilities { return m.capabilities }
func (m CatalogModel) Source() CatalogSource           { return m.source }

// Catalog sentinels. They live here rather than in errors.go because they are
// the vocabulary of this aggregate; the codes they carry are the §8 ones.
var (
	ErrModelNotFound = NewNotFoundError("model not found")
	ErrModelExists   = NewConflictError("a model with this provider and id already exists")
)
