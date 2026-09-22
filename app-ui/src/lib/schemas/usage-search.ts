// Reading the Usage screen's filters out of the URL (docs/SPEC-UI/001-SPEC-UI.md §6.5, §8.4.2).
//
// The URL is the source of truth for these filters, which is what makes a filtered view shareable and what
// makes back and forward navigation restore it. That puts two obligations here: a hand-edited URL has to
// resolve into a usable query rather than a failed request, and a correction has to be visible rather than
// silent (§7.1.1).
//
// An unknown *key* is ignored, because a foreign query parameter is not the panel's business. A known key
// with a value the panel cannot use falls back to its default and records a notice, which is the visible
// half of that rule.
//
// The page size is the one parameter the panel corrects rather than merely reports. The screen reads a
// fixed 25 rows while the API accepts up to 100 (SPEC-API §4), so a `per_page` a shared link carries would
// otherwise leave the URL, the notice, and the rows actually read disagreeing three ways.

import type { z } from 'zod';
import {
	pageNumber,
	perPage,
	schemaRequestStatus,
	searchText,
	type RequestStatus
} from './primitives';
import {
	DEFAULT_USAGE_BREAKDOWN,
	DEFAULT_USAGE_ORDER,
	DEFAULT_USAGE_PERIOD,
	schemaUsageBreakdown,
	schemaUsageOrder,
	schemaUsagePeriod,
	schemaUsageSort,
	type UsageBreakdown,
	type UsageOrder,
	type UsagePeriod,
	type UsageSort
} from './usage';
import { USAGE_RECORDS_PAGE_SIZE } from './usage-view';

/** The filter state the Usage tabs read out of the URL. */
export type UsageSearch = {
	period: UsagePeriod;
	/** The breakdown the table shows, or `none` when the operator turned it off (draft 014 F1). */
	groupBy: UsageBreakdown;
	/** The column the breakdown table is ordered by, or empty for the API's own order. */
	sort: UsageSort | '';
	order: UsageOrder;
	status: RequestStatus | '';
	endpointId: string;
	providerId: string;
	gatewayKeyId: string;
	model: string;
	query: string;
	page: number;
	/** The page size the URL asked for, or the screen's own when the URL said nothing readable. */
	perPageRequested: number;
	/** What the panel had to correct about the page size, in the operator's terms. Null when the URL was usable. */
	perPageNotice: string | null;
	/** What the panel had to correct in the URL, in the operator's terms. Empty when the URL was usable. */
	notices: string[];
};

// One known parameter, parsed against its own schema.
function pick<T>(
	raw: string | null,
	schema: z.ZodType<T>,
	fallback: T,
	label: string,
	fallbackText: string,
	notices: string[]
): T {
	if (raw === null || raw === '') return fallback;

	const parsed = schema.safeParse(raw);
	if (parsed.success) return parsed.data;

	notices.push(`The ${label} "${raw}" is not one the panel offers, so ${fallbackText} is shown.`);
	return fallback;
}

// Free-text filters are bounded at 200 characters (§7.2). The rejected value is not echoed, because the
// reason it failed is its length and printing a pasted blob back into a notice helps nobody.
function pickText(raw: string | null, notices: string[]): string {
	if (raw === null || raw === '') return '';

	const parsed = searchText.safeParse(raw);
	if (parsed.success) return parsed.data;

	notices.push('A filter value was longer than 200 characters, so it was left out.');
	return '';
}

/**
 * The page size the URL asked for, as far as the panel can read it.
 *
 * Only the notice uses this. The table reads `USAGE_RECORDS_PAGE_SIZE` whatever the URL says, so an
 * unreadable value resolves to that size here too and the notice says so in words.
 */
function perPageRequested(params: URLSearchParams): number {
	const parsed = perPage.safeParse(params.get('per_page') ?? String(USAGE_RECORDS_PAGE_SIZE));

	return parsed.success ? parsed.data : USAGE_RECORDS_PAGE_SIZE;
}

/**
 * What the panel has to say about the URL's page size, or null when there is nothing to correct.
 *
 * The value is not echoed when the schema refuses it: a hand-edited URL can hold a long string, and
 * printing it back into a notice helps nobody. The number the screen reads is named in every case.
 */
function perPageNotice(params: URLSearchParams): string | null {
	const raw = params.get('per_page');
	if (raw === null) return null;

	const parsed = perPage.safeParse(raw);
	if (parsed.success && parsed.data === USAGE_RECORDS_PAGE_SIZE) return null;

	const reason = parsed.success
		? `The URL asked for ${parsed.data} records per page`
		: 'The page size in this URL is not one the panel can read';

	return `${reason}, so this screen reads ${USAGE_RECORDS_PAGE_SIZE} and the parameter was removed.`;
}

/**
 * Reads the URL into the filter state, correcting what it cannot use and reporting each correction.
 */
export function parseUsageSearch(params: URLSearchParams): UsageSearch {
	const notices: string[] = [];

	return {
		period: pick(
			params.get('period'),
			schemaUsagePeriod,
			DEFAULT_USAGE_PERIOD,
			'period',
			'the last 24 hours',
			notices
		),
		groupBy: pick(
			params.get('group_by'),
			schemaUsageBreakdown,
			DEFAULT_USAGE_BREAKDOWN,
			'group_by value',
			'the breakdown by model',
			notices
		),
		sort: pick(
			params.get('sort'),
			schemaUsageSort,
			'',
			'sort value',
			"the API's own order",
			notices
		),
		order: pick(
			params.get('order'),
			schemaUsageOrder,
			DEFAULT_USAGE_ORDER,
			'sort order',
			'ascending order',
			notices
		),
		status: pick(params.get('status'), schemaRequestStatus, '', 'status', 'any status', notices),
		endpointId: pickText(params.get('endpoint_id'), notices),
		providerId: pickText(params.get('provider_id'), notices),
		gatewayKeyId: pickText(params.get('gateway_key_id'), notices),
		model: pickText(params.get('model'), notices),
		query: pickText(params.get('q'), notices),
		page: pick(params.get('page'), pageNumber, 1, 'page', 'page 1', notices),
		perPageRequested: perPageRequested(params),
		perPageNotice: perPageNotice(params),
		notices
	};
}

/**
 * The URL a correction asks for, or null when the URL carries no page size to correct.
 *
 * The caller navigates to this rather than editing the address in place, because the URL is the source of
 * truth for the screen's filters (§8.4.2): one path writes it, and that path is a navigation.
 */
export function cleanedUsageSearch(current: URLSearchParams): URLSearchParams | null {
	if (perPageNotice(current) === null) return null;

	const next = new URLSearchParams(current);
	next.delete('per_page');

	return next;
}

/**
 * True when any filter other than the period is set.
 *
 * The period is excluded on purpose: it always has a value, so counting it would make "Clear filters"
 * appear on a screen where nothing is filtered, and a control that appears to do something when there is
 * nothing to clear is the kind of control R-26 forbids.
 */
export function usageFiltersApplied(search: UsageSearch): boolean {
	return (
		search.status !== '' ||
		search.endpointId !== '' ||
		search.providerId !== '' ||
		search.gatewayKeyId !== '' ||
		search.model !== '' ||
		search.query !== ''
	);
}

/**
 * The next URL after a filter changes.
 *
 * A filter change clears the page, because staying on page 7 of a narrower result would show an empty
 * table for a filter that did match rows. Changing the page itself does not, which is the one case where
 * the operator asked to move.
 */
export function nextUsageSearch(
	current: URLSearchParams,
	key: string,
	value: string
): URLSearchParams {
	const next = new URLSearchParams(current);

	if (value === '') next.delete(key);
	else next.set(key, value);

	if (key !== 'page') next.delete('page');

	return next;
}
