// Arming the accounts the poll queue has never held.
//
// @file      internal/service/quota_published_seed.go
// @for       Finding active accounts with no scheduling row and arming them for the sweep.
// @uses      context, internal/domain, internal/repository.
// @reason    A provider answer can only be cached for an account the queue knows about, and the
//
//	queue is filled from the endpoint list rather than from the counters, so an account that has
//	routed nothing is still polled. Two bounded queries per tick, whatever the fleet size.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// Seed arms one page of active accounts that have no scheduling row, then moves the cursor, so
// successive ticks walk the whole fleet. This is the cheapest honest way to make a new endpoint
// pollable: two bounded queries per tick — a page list and a batched existence read — whatever
// the fleet size, so no tick scans every endpoint and the sweep's provider budget stays intact.
// The existence test is free because the cache read's contract is that an endpoint with no state
// row is absent from it.
//
// The row comes from RecordAttempt, the port's only row creator, stamped due at this instant so
// the next sweep picks the account up inside its budget. Only families the registry marks
// usage:true are armed; the rest would fill the queue with polls livePublishedUsage refuses
// before dialling. Arming is one attempt per new endpoint because the port has no batch writer,
// bounded by SeedPageSize per tick — a batched SeedScheduling is the follow-up.
func (w *QuotaPublishedWorker) Seed(ctx context.Context) int {
	page := w.seedPage + 1
	listCtx, cancel := context.WithTimeout(ctx, w.policy.StoreTimeout)
	endpoints, total, err := w.endpoints.List(listCtx,
		repository.EndpointFilter{Status: string(domain.UpstreamEndpointActive)},
		repository.PageQuery{Page: page, PerPage: w.policy.SeedPageSize})
	cancel()
	if err != nil {
		w.logger.Warn("published quota worker could not list endpoints to schedule", "page", page, "error", err)
		return 0
	}
	// The cursor wraps on the last page, so a shrinking fleet is walked from the top again.
	if int64(page)*int64(w.policy.SeedPageSize) >= total {
		w.seedPage = 0
	} else {
		w.seedPage = page
	}

	missing := w.unscheduledEndpoints(ctx, endpoints)
	now := w.clock()
	for _, endpoint := range missing {
		w.schedule(ctx, domain.PublishedAttempt{
			EndpointID: endpoint.ID(), ProviderID: endpoint.ProviderID(), AttemptedAt: now,
		}, 0)
	}
	return len(missing)
}

// unscheduledEndpoints keeps the accounts the registry can answer for that the cache has never
// heard of, in one batched read for the page rather than one per account (§1.7).
func (w *QuotaPublishedWorker) unscheduledEndpoints(ctx context.Context, endpoints []domain.UpstreamEndpoint) []domain.UpstreamEndpoint {
	candidates := make([]domain.UpstreamEndpoint, 0, len(endpoints))
	ids := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		entry, known := w.providers.Provider(endpoint.ProviderID())
		if !known || !entry.Features.Usage {
			continue
		}
		candidates, ids = append(candidates, endpoint), append(ids, endpoint.ID())
	}
	if len(ids) == 0 {
		return nil
	}
	readCtx, cancel := context.WithTimeout(ctx, w.policy.StoreTimeout)
	cached, err := w.store.ListPublishedByEndpointIDs(readCtx, ids)
	cancel()
	if err != nil {
		w.logger.Warn("published quota worker could not check which endpoints are scheduled",
			"candidates", len(ids), "error", err)
		return nil
	}

	missing := make([]domain.UpstreamEndpoint, 0, len(candidates))
	for _, endpoint := range candidates {
		if _, scheduled := cached[endpoint.ID()]; !scheduled {
			missing = append(missing, endpoint)
		}
	}
	return missing
}
