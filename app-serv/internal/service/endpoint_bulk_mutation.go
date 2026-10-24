// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_bulk_mutation.go
// @for       Resolving one bulk selector against stored rows, and the bulk-status action that parks or releases the set it names (SPEC-API-001 §7.5, §8.1).
// @uses      context, fmt, strings, time, internal/domain, internal/repository.
// @reason    A bulk action is the one management path where a wrong set destroys rows nobody pointed at, so the resolution happens here rather than in a browser iterating a page: the server counts what its own selector matches, refuses a set wider than the ceiling with the number it found, and refuses to act at all when the count the caller confirmed is not the count the rows give. Writing the decision once, in front of every bulk route, is what keeps a stale selection from becoming a stale mutation.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// MaxBulkTargets is the ceiling on a resolved set. It is the §1.7 no-unbounded-query
// rule applied to a mutation: an operator who selects every account across every
// provider is asking for more rows than one transaction should hold, and the refusal
// names the number so they know how far to narrow it.
const MaxBulkTargets = 2000

// bulkResolvePerPage is the page the resolver walks. It matches the OAuth sweep's page
// rather than the panel's, because this loop reads whole aggregates to apply the
// aggregate's own methods, not a display projection.
const bulkResolvePerPage = 100

// BulkSelector is a resolved selector: which rows one bulk action means, after the
// boundary refused an empty or doubled one. ProviderID and AuthType narrow whichever
// form is set, and an empty ProviderID with All names every provider, native or custom.
type BulkSelector struct {
	IDs        []string
	Tag        domain.Tag
	AuthType   domain.UpstreamAuthType
	ProviderID string
	All        bool
}

// BulkRowResult is one row's outcome. A nil Error means the action landed on it.
type BulkRowResult struct {
	Index int
	ID    string
	Error error
}

// BulkOutcome is what a bulk action did, in the shape every bulk route answers with.
// Matched is the server's own count of the set, and Changed counts the rows the action
// actually moved, which is lower whenever a row was already in the state asked for.
type BulkOutcome struct {
	Matched int
	Changed int
	Results []BulkRowResult
}

// HasTargets reports whether the selector names a set. An empty one is refused here as
// well as at the boundary, because "no selection means everything" is the failure this
// feature exists to make impossible, and a caller that reaches the service without going
// through the wire must not be able to express it.
func (s BulkSelector) HasTargets() bool {
	return s.All || s.Tag != "" || len(s.IDs) > 0
}

// filter builds the storage narrowing this selector resolves through. The tag and auth
// type are passed as the wire spells them, because the selector was normalized at the
// boundary and storage compares canonical values only.
func (s BulkSelector) filter() repository.EndpointFilter {
	return repository.EndpointFilter{
		ProviderID: s.ProviderID,
		IDs:        s.IDs,
		Tag:        s.Tag.String(),
		AuthType:   string(s.AuthType),
	}
}

// resolveBulk loads every endpoint the selector names and returns them as aggregates,
// so the caller applies the domain's own transitions rather than patching columns.
//
// The count the caller confirmed is checked against the count the rows give before any
// aggregate is loaded, and a set past the ceiling is refused with the number it
// resolved to. Both checks answer about the same snapshot the mutation then writes: a
// list that changed between the operator's render and their click fails the count rather
// than acting on the new set.
func (s *EndpointService) resolveBulk(ctx context.Context, sel BulkSelector, confirmCount int) ([]domain.UpstreamEndpoint, error) {
	if !sel.HasTargets() {
		return nil, domain.NewBulkSelectorRequiredError(
			"select rows by ids, name one tag, or send all=true")
	}
	filter := sel.filter()
	perPage := bulkResolvePerPage
	if len(sel.IDs) > 0 && len(sel.IDs) < perPage {
		perPage = len(sel.IDs)
	}
	page, total, err := s.store.List(ctx, filter, repository.PageQuery{Page: 1, PerPage: perPage})
	if err != nil {
		return nil, err
	}
	if total > MaxBulkTargets {
		return nil, domain.NewBulkTargetTooLargeError(fmt.Sprintf(
			"this selector resolves to %d rows, and one action may address at most %d; narrow it to a provider or a tag",
			total, MaxBulkTargets))
	}
	if int64(confirmCount) != total {
		return nil, domain.NewBulkTargetCountMismatchError(fmt.Sprintf(
			"the selector now resolves to %d rows, not the %d this action confirmed; refresh the list and repeat",
			total, confirmCount))
	}

	resolved := make([]domain.UpstreamEndpoint, 0, int(total))
	seen := make(map[string]struct{}, int(total))
	for _, endpoint := range page {
		seen[endpoint.ID()] = struct{}{}
		resolved = append(resolved, endpoint)
	}
	for pageNo := 2; int64(len(resolved)) < total; pageNo++ {
		next, _, err := s.store.List(ctx, filter, repository.PageQuery{Page: pageNo, PerPage: perPage})
		if err != nil {
			return nil, err
		}
		if len(next) == 0 {
			break
		}
		for _, endpoint := range next {
			if _, dup := seen[endpoint.ID()]; dup {
				continue
			}
			seen[endpoint.ID()] = struct{}{}
			resolved = append(resolved, endpoint)
		}
	}
	return resolved, nil
}

// BulkStatus parks or releases every account the selector names.
//
// A park with no stated window lasts as long as the provider's own reset cadence, which
// is what makes the switch adaptive rather than a decision the operator has to reverse by
// hand: an account disabled at a vendor that publishes a five-hour allowance is routable
// again in five hours without anyone touching it. Releasing clears the window, the health
// park and the breaker run, because an account handed back by an operator starts honest.
func (s *EndpointService) BulkStatus(ctx context.Context, sel BulkSelector, status string, windowHours int, confirmCount int) (BulkOutcome, error) {
	parsed, err := domain.ParseUpstreamEndpointStatus(status)
	if err != nil {
		return BulkOutcome{}, err
	}
	endpoints, err := s.resolveBulk(ctx, sel, confirmCount)
	if err != nil {
		return BulkOutcome{}, err
	}

	now := s.clock()
	changed := make([]domain.UpstreamEndpoint, 0, len(endpoints))
	outcome := BulkOutcome{Matched: len(endpoints), Results: make([]BulkRowResult, 0, len(endpoints))}
	for i, endpoint := range endpoints {
		before := readPark(endpoint)
		if parsed == domain.UpstreamEndpointDisabled {
			endpoint.Disable(s.disableDeadline(ctx, endpoint.ProviderID(), windowHours, now), now)
		} else {
			endpoint.Enable(now)
		}
		outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: endpoint.ID()})
		if readPark(endpoint) != before {
			changed = append(changed, endpoint)
		}
	}
	if len(changed) > 0 {
		if err := s.store.UpdateBulk(ctx, changed); err != nil {
			return BulkOutcome{}, err
		}
	}
	outcome.Changed = len(changed)
	return outcome, nil
}

// parkState is the part of an account's switch a status write can change: which way it is
// set and the deadline it carries. Comparing it is how a bulk action counts what it moved
// without writing rows that already said what the operator asked.
type parkState struct {
	status domain.UpstreamEndpointStatus
	until  time.Time
}

func readPark(endpoint domain.UpstreamEndpoint) parkState {
	state := parkState{status: endpoint.Status()}
	if until := endpoint.DisabledUntil(); until != nil {
		state.until = *until
	}
	return state
}

// disableDeadline answers when a park opened at `now` releases itself. A stated window
// wins; otherwise the provider's own cadence does, read from the same policy that sizes
// the health ladder so one number cannot disagree with the other.
func (s *EndpointService) disableDeadline(ctx context.Context, providerID string, windowHours int, now time.Time) time.Time {
	if windowHours > 0 {
		return now.Add(time.Duration(windowHours) * time.Hour)
	}
	return now.Add(s.healthPolicy(ctx, providerID).DisableWindow())
}
