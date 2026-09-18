// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_keys.go
// @for       Upstream key CRUD and health on an endpoint: add, list, patch, and
//
//	remove, plus their batch forms (SPEC-API-001 §7.5).
//
// @uses      internal/domain, internal/repository, context, strings.
// @reason    §7.5 makes a key a child of the endpoint aggregate, so every mutation
//
//	runs through the root's own methods rather than writing a key row
//	directly (AGENTS.md §2.2) — that is what enforces "an api_key endpoint
//	keeps at least one active key". The credential is sealed here so the
//	aggregate never meets plaintext, and the sealed value is never read
//	back out: a response carries the hint the aggregate already holds.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// KeyPatch is a partial change to one key. A nil field means "leave unchanged";
// a nil Value keeps the stored credential, because the value is write-only on the
// wire (§7.5).
type KeyPatch struct {
	Label    *string
	Value    *string
	Priority *int
	Status   *string
}

// AddKey appends one credential to an existing endpoint.
//
// The credential is sealed before the aggregate sees it and the hint is derived
// from the plaintext in the same step, so the plaintext exists only for the
// duration of this call (SPEC-API-001 §6).
func (s *EndpointService) AddKey(ctx context.Context, endpointID string, input KeyInput) (domain.UpstreamKey, error) {
	endpoint, err := s.store.GetByID(ctx, endpointID)
	if err != nil {
		return domain.UpstreamKey{}, err
	}
	if len(endpoint.Keys()) >= maxKeysPerEndpoint {
		return domain.UpstreamKey{}, domain.NewValidationError("an endpoint may hold at most 100 keys")
	}
	if err := s.attachKey(&endpoint, input, s.clock()); err != nil {
		return domain.UpstreamKey{}, err
	}
	key := endpoint.Keys()[len(endpoint.Keys())-1]
	if err := s.store.AddKey(ctx, endpointID, key); err != nil {
		return domain.UpstreamKey{}, err
	}
	return key, nil
}

// ListKeys returns one page of an endpoint's keys, hints and health only.
//
// The endpoint is loaded whole because the aggregate owns selection, then paged
// in memory: a key set is bounded at maxKeysPerEndpoint, so a page slice is the
// truthful expression of "one page of a small, bounded child collection" rather
// than an unbounded read.
func (s *EndpointService) ListKeys(ctx context.Context, endpointID string, page, perPage int) ([]domain.UpstreamKey, int64, error) {
	endpoint, err := s.store.GetByID(ctx, endpointID)
	if err != nil {
		return nil, 0, err
	}
	keys := endpoint.Keys()
	total := int64(len(keys))
	start := repository.PageQuery{Page: page, PerPage: perPage}.Offset()
	if start >= len(keys) {
		return []domain.UpstreamKey{}, total, nil
	}
	end := clamp(start+perPage, start, len(keys))
	return keys[start:end], total, nil
}

// UpdateKey applies a PATCH to one key. A supplied value is sealed and replaces
// the stored credential; an omitted one leaves it untouched. Health earned by the
// prior credential is preserved by the aggregate's own rule, because a rotated
// credential is still the same account.
func (s *EndpointService) UpdateKey(ctx context.Context, endpointID, keyID string, patch KeyPatch) (domain.UpstreamKey, error) {
	endpoint, err := s.store.GetByID(ctx, endpointID)
	if err != nil {
		return domain.UpstreamKey{}, err
	}
	existing := endpoint.Key(keyID)
	if existing.ID() == "" {
		return domain.UpstreamKey{}, domain.NewNotFoundError("upstream key not found")
	}

	label := existing.Label()
	if patch.Label != nil {
		label = strings.TrimSpace(*patch.Label)
	}
	priority := existing.Priority()
	if patch.Priority != nil {
		priority = *patch.Priority
	}
	status := ""
	if patch.Status != nil {
		status = *patch.Status
	}
	sealed, hint, err := s.sealReplacement(patch.Value)
	if err != nil {
		return domain.UpstreamKey{}, err
	}

	updated, err := endpoint.UpdateKey(keyID, label, sealed, hint, priority, status, s.clock())
	if err != nil {
		return domain.UpstreamKey{}, err
	}
	if err := s.store.UpdateKey(ctx, updated); err != nil {
		return domain.UpstreamKey{}, err
	}
	return updated, nil
}

// sealReplacement seals a replacement credential when the caller supplied one.
// An omitted value yields an empty sealed value and an empty hint, which is the
// signal the aggregate reads as "keep the stored credential".
func (s *EndpointService) sealReplacement(value *string) (sealed, hint string, err error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "", "", nil
	}
	plaintext := strings.TrimSpace(*value)
	sealed, err = s.sealer.Seal(plaintext)
	if err != nil {
		return "", "", domain.NewInternalError("the credential could not be stored")
	}
	return sealed, domain.MaskSecret(plaintext), nil
}

// RemoveKey deletes one key. The endpoint's own rule refuses a removal that would
// leave an api_key endpoint unable to route, and that refusal is a CONFLICT the
// panel already anticipates (§7.5).
func (s *EndpointService) RemoveKey(ctx context.Context, endpointID, keyID string) error {
	endpoint, err := s.store.GetByID(ctx, endpointID)
	if err != nil {
		return err
	}
	if err := endpoint.RemoveKey(keyID, s.clock()); err != nil {
		return err
	}
	if err := s.store.Update(ctx, endpoint); err != nil {
		return err
	}
	return s.store.DeleteKey(ctx, endpointID, keyID)
}

// AddKeyBatch appends several credentials to one endpoint, all or nothing.
//
// The whole batch is validated before anything is stored: a duplicate label inside
// the batch is a VALIDATION_ERROR (§7.5), and the keys are attached to an
// in-memory aggregate first so the aggregate's own rules — including the
// per-endpoint label uniqueness — run before a single statement.
func (s *EndpointService) AddKeyBatch(ctx context.Context, endpointID string, inputs []KeyInput) ([]domain.UpstreamKey, error) {
	endpoint, err := s.store.GetByID(ctx, endpointID)
	if err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, domain.NewValidationError("keys is required")
	}
	if len(endpoint.Keys())+len(inputs) > maxKeysPerEndpoint {
		return nil, domain.NewValidationError("an endpoint may hold at most 100 keys")
	}

	before := len(endpoint.Keys())
	now := s.clock()
	for i, input := range inputs {
		if err := s.attachKey(&endpoint, input, now); err != nil {
			return nil, rowError(i, err)
		}
	}
	added := endpoint.Keys()[before:]
	if err := s.store.AddKeys(ctx, endpointID, added); err != nil {
		return nil, err
	}
	return added, nil
}

// rowError attributes a refused batch row to its index when the service itself
// detected the problem, so the handler can report {index, error} per row exactly
// as it does for a storage-side refusal.
func rowError(index int, err error) error {
	return &bulkRowError{index: index, err: err}
}

// bulkRowError is the service-side counterpart of the repository's row-attributed
// error. It implements the same BulkRowIndexer method, so the handler treats both
// the same way.
type bulkRowError struct {
	index int
	err   error
}

// Error renders the cause, so the value reads correctly in a log line.
func (e *bulkRowError) Error() string { return e.err.Error() }

// Unwrap exposes the cause to errors.Is and errors.As.
func (e *bulkRowError) Unwrap() error { return e.err }

// BulkRowIndex reports the offending row's position.
func (e *bulkRowError) BulkRowIndex() (int, bool) { return e.index, true }
