// Stub for the quota collection's account page.
//
// @file      internal/handler/quota_account_stub_test.go
// @for       Answers the account page of the quota collection from the handler fixture's in-memory set.
// @uses      context, internal/domain, sort, testing.
// @reason    A quota page is grouped by accounts, not by counters, and the route tests need the same grouping the SQL applies to answer them. It lives apart from the window paging stub because it models a different read, and the two must agree on which providers a page number means, an agreement this file reproduces rather than reuses.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-02
package handler

import (
	"context"
	"sort"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// PageAccountsByProvider answers the account list the same way the SQL does: groups are
// the distinct provider ids over accounts UNION counted windows, ordered by each group's
// smallest endpoint id, sliced to the page. Windows contribute accounts too, so a stub
// that seeds only windows still sees its accounts appear.
func (r *stubQuotaRepo) PageAccountsByProvider(_ context.Context, page, perPage int) ([]domain.QuotaAccount, int64, error) {
	source := append([]domain.QuotaAccount{}, r.accounts...)
	for _, window := range r.windows {
		source = append(source, domain.QuotaAccount{EndpointID: window.EndpointID(), ProviderID: window.ProviderID()})
	}

	unique := map[string]domain.QuotaAccount{}
	for _, account := range source {
		unique[account.EndpointID+"|"+account.ProviderID] = account
	}

	first := map[string]string{}
	for _, account := range unique {
		if seen, ok := first[account.ProviderID]; !ok || account.EndpointID < seen {
			first[account.ProviderID] = account.EndpointID
		}
	}
	order := make([]string, 0, len(first))
	for providerID := range first {
		order = append(order, providerID)
	}
	sort.Slice(order, func(i, j int) bool { return first[order[i]] < first[order[j]] })

	total := int64(len(order))
	start := (page - 1) * perPage
	if start >= len(order) {
		return []domain.QuotaAccount{}, total, nil
	}
	keep := map[string]bool{}
	for _, providerID := range order[start:min(start+perPage, len(order)) /* builtin min */] {
		keep[providerID] = true
	}

	out := make([]domain.QuotaAccount, 0)
	for _, providerID := range order {
		if !keep[providerID] {
			continue
		}
		group := make([]domain.QuotaAccount, 0)
		for _, account := range unique {
			if account.ProviderID == providerID {
				group = append(group, account)
			}
		}
		sort.Slice(group, func(i, j int) bool { return group[i].EndpointID < group[j].EndpointID })
		out = append(out, group...)
	}
	return out, total, nil
}
