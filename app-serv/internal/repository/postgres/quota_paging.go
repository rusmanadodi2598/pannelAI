// The quota screen's paging reads: which provider groups a page carries, and what
// belongs to them.
//
// @file      internal/repository/postgres/quota_paging.go
// @for       Paging the quota collection over provider groups and listing the accounts in the page's groups.
// @uses      context, internal/domain, internal/repository/postgres, pgxpool, time.
// @reason    A card is a provider and its accounts. Selecting groups from counted
//
//	windows alone hid every account that had not routed a request yet, a
//	provider could be configured, polled by the worker, and publishing real
//	quota, and still render no card at all. Both reads here select groups
//	from the same source, so the window page and the account page can never
//	disagree about which providers a given page number means.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-02
package postgres

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// quotaMaxRowsPerPage caps the rows one page of the collection read may return.
// Paging is by provider group, and a group can hold any number of windows, so
// without this a single provider with a long history makes the page unbounded on
// a live screen path (AGENTS.md §1.7). It is generous enough that the panel's
// cards are complete for every account count the deployment realistically holds.
const quotaMaxRowsPerPage = 1000

// quotaAccountSource is every (provider, endpoint) pair the quota screen knows
// about: the configured accounts, plus the window rows' own endpoints, so the
// credential-free virtual lane and an endpoint deleted after being counted still
// form a group. `provider_id` is a property of the endpoint: joined, never copied
// into the counter row.
const quotaAccountSource = `
    SELECT coalesce(e.provider_id, '') AS provider_id, e.id AS endpoint_id
      FROM upstream_endpoints e
    UNION
    SELECT coalesce(e.provider_id, '') AS provider_id, w.endpoint_id AS endpoint_id
      FROM quota_windows w
      LEFT JOIN upstream_endpoints e ON e.id = w.endpoint_id`

// quotaGroupCount counts provider groups over that source. The page unit is the
// group (docs/PORT/006-PORT-QUOTA-PAGING.md D1), so this is the number the meta
// block reports and the number the card pager walks.
const quotaGroupCount = `SELECT count(*) FROM (
    SELECT provider_id FROM (` + quotaAccountSource + `) u
     GROUP BY provider_id
) groups`

// quotaPageCTE names the page's groups, ordered by each group's smallest endpoint
// id. That ordering is the first-seen order the client's grouping reproduces, so
// two reads of an unchanged table render the cards in the same order. Both this
// file's queries share it rather than restating it, which is what keeps the window
// page and the account page describing the same page.
const quotaPageCTE = `WITH page AS (
    SELECT provider_id, min(endpoint_id) AS first_endpoint
      FROM (` + quotaAccountSource + `) u
     GROUP BY provider_id
     ORDER BY min(endpoint_id) ASC
     LIMIT $1::int OFFSET $2::int
)`

// PageWindowsByProvider returns one page of the collection read: every window of
// the page's provider groups, with the total group count. The count and the page
// are two statements: a window written between them shifts a boundary at worst
// and never loses a group the count promised.
// The row ceiling is separate from the group ceiling: paging by group bounds the
// providers per round trip, not the rows they hold, and the LIMIT is what §1.7
// asks of a request-serving read. A group beyond the ceiling is served in key
// order and truncated: a visible gap in one card, not an unbounded read.
func (r *QuotaRepository) PageWindowsByProvider(ctx context.Context, page, perPage int) ([]domain.QuotaWindow, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, quotaGroupCount).Scan(&total); err != nil {
		return nil, 0, translateQuotaError(err)
	}

	pageQ := quotaPageCTE + `
  SELECT ` + quotaWindowColumns + `
    FROM quota_windows w
    LEFT JOIN upstream_endpoints e ON e.id = w.endpoint_id
   WHERE coalesce(e.provider_id, '') IN (SELECT provider_id FROM page)
   ORDER BY w.endpoint_id ASC, w."window" ASC
   LIMIT $3::int`

	rows, err := r.pool.Query(ctx, pageQ, perPage, (page-1)*perPage, quotaMaxRowsPerPage)
	if err != nil {
		return nil, 0, translateQuotaError(err)
	}
	defer rows.Close()

	windows := make([]domain.QuotaWindow, 0, perPage)
	for rows.Next() {
		window, err := scanQuotaWindow(rows)
		if err != nil {
			return nil, 0, err
		}
		windows = append(windows, window)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translateQuotaError(err)
	}
	return windows, total, nil
}

// PageAccountsByProvider returns every account in the page's provider groups, in
// group order then endpoint id, with the same total group count the window page
// reports. Listing accounts rather than implying them from counted rows is what
// makes a card exist for an endpoint that has served nothing: the screen cannot
// show a provider's published quota for an account it never learns about. The
// group ceiling bounds the round trips and quotaMaxRowsPerPage bounds the rows,
// which is what §1.7 asks of a request-serving read.
func (r *QuotaRepository) PageAccountsByProvider(ctx context.Context, page, perPage int) ([]domain.QuotaAccount, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, quotaGroupCount).Scan(&total); err != nil {
		return nil, 0, translateQuotaError(err)
	}

	pageQ := quotaPageCTE + `
  SELECT u.endpoint_id, u.provider_id
    FROM (` + quotaAccountSource + `) u
    JOIN page p ON p.provider_id = u.provider_id
   ORDER BY p.first_endpoint ASC, u.endpoint_id ASC
   LIMIT $3::int`

	rows, err := r.pool.Query(ctx, pageQ, perPage, (page-1)*perPage, quotaMaxRowsPerPage)
	if err != nil {
		return nil, 0, translateQuotaError(err)
	}
	defer rows.Close()

	accounts := make([]domain.QuotaAccount, 0, perPage)
	for rows.Next() {
		var account domain.QuotaAccount
		if err := rows.Scan(&account.EndpointID, &account.ProviderID); err != nil {
			return nil, 0, translateQuotaError(err)
		}
		if err := account.Validate(); err != nil {
			return nil, 0, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translateQuotaError(err)
	}
	return accounts, total, nil
}
