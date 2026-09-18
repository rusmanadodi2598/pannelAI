// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_create.go
// @for       Building one endpoint for storage: validating its provider, sealing
//
//	every credential, and renumbering siblings when a priority changes.
//
// @uses      internal/domain, context, strconv, strings, time.
// @reason    SPEC-API-001 §8.1 settles one create shape for the single route and
//
//	for each element of the bulk route, so the build step is shared rather
//	than duplicated per entry point; the sibling renumber lives beside it
//	because a priority is only meaningful against the provider's other
//	endpoints. It is a separate file because AGENTS.md §1.1 caps a file at
//	250 lines and the lifecycle surface already fills one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// buildEndpoint validates one create input and returns the aggregate ready to
// persist.
func (s *EndpointService) buildEndpoint(in CreateInput, now time.Time) (domain.UpstreamEndpoint, error) {
	providerID := strings.TrimSpace(in.ProviderID)
	if providerID == "" {
		return domain.UpstreamEndpoint{}, domain.NewValidationError("provider_id is required")
	}
	if _, ok := s.index.Provider(providerID); !ok {
		return domain.UpstreamEndpoint{}, domain.NewValidationError("unknown provider_id: " + providerID)
	}

	priority := in.Priority
	if priority < 1 {
		priority = 1
	}
	endpoint, err := domain.NewUpstreamEndpoint(domain.IDPrefixUpstreamEndpoint+domain.NewULID(now),
		providerID, in.Label, in.AuthType, priority, now)
	if err != nil {
		return domain.UpstreamEndpoint{}, err
	}

	if in.AuthType == domain.UpstreamAuthAPIKey && len(in.Keys) == 0 {
		return domain.UpstreamEndpoint{}, domain.NewValidationError("an api_key endpoint requires at least one key")
	}
	if len(in.Keys) > maxKeysPerEndpoint {
		return domain.UpstreamEndpoint{}, domain.NewValidationError("an endpoint may hold at most 100 keys")
	}
	for _, input := range in.Keys {
		if err := s.attachKey(&endpoint, input, now); err != nil {
			return domain.UpstreamEndpoint{}, err
		}
	}
	return endpoint, nil
}

// attachKey seals one credential and attaches it to the aggregate. An unnamed key
// gets a positional label so two unnamed keys in one request stay
// distinguishable, and the aggregate's own duplicate-label rule still applies.
func (s *EndpointService) attachKey(endpoint *domain.UpstreamEndpoint, input KeyInput, now time.Time) error {
	value := strings.TrimSpace(input.Value)
	if value == "" {
		return domain.NewValidationError("key value is required")
	}
	label := strings.TrimSpace(input.Label)
	if label == "" {
		label = defaultKeyLabel(len(endpoint.Keys()) + 1)
	}
	sealed, err := s.sealer.Seal(value)
	if err != nil {
		return domain.NewInternalError("the credential could not be stored")
	}
	if _, err := endpoint.AddKey(label, sealed, domain.MaskSecret(value), input.Priority, now); err != nil {
		return err
	}
	return nil
}

// defaultKeyLabel names an unnamed key by its position, which is what the keys
// table shows and what keeps two keys of one request apart.
func defaultKeyLabel(position int) string {
	return "key-" + strconv.Itoa(position)
}

// reorderSiblings renumbers a provider's endpoints so the changed priority lands
// where the operator put it and no two endpoints share a slot.
//
// The list is read, the moved endpoint is lifted to its new position, and the
// whole order is written back: sending only the moved endpoint's new number would
// leave a sibling holding it too, which is the collision Reorder exists to refuse.
func (s *EndpointService) reorderSiblings(ctx context.Context, moved domain.UpstreamEndpoint) error {
	ids, err := s.store.IDsByProvider(ctx, moved.ProviderID())
	if err != nil {
		return err
	}
	ordered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != moved.ID() {
			ordered = append(ordered, id)
		}
	}
	target := clamp(moved.Priority(), 1, len(ordered)+1)
	return s.store.Reorder(ctx, moved.ProviderID(), insertAt(ordered, target-1, moved.ID()))
}

// insertAt places value at index, shifting the tail right. A position past the end
// appends, which is the truthful result of "after the last one"; the caller clamps
// first, so that branch is reachable only for an empty list.
//
// The copy runs from the end backwards, so it reads each element before writing
// over it. Forward-copying would duplicate the element at index into the first
// tail slot and lose the rest of the tail's original values.
func insertAt(list []string, index int, value string) []string {
	if index >= len(list) {
		return append(list, value)
	}
	list = append(list, "")
	for i := len(list) - 1; i > index; i-- {
		list[i] = list[i-1]
	}
	list[index] = value
	return list
}

// clamp bounds value to [low, high].
func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
