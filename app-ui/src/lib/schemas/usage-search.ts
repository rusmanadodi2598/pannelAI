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

import type { z } from 'zod';
import { pageNumber, schemaRequestStatus, searchText, type RequestStatus } from './primitives';
import {
	DEFAULT_USAGE_PERIOD,
	schemaUsageGroupBy,
	schemaUsagePeriod,
	type UsageGroupBy,
	type UsagePeriod
} from './usage';
import { USAGE_RECORDS_PAGE_SIZE } from './usage-view';

/** The filter state the Usage tabs read out of the URL. */
export type UsageSearch = {
	period: UsagePeriod;
	groupBy: UsageGroupBy | '';
	status: RequestStatus | '';
	endpointId: string;
	model: string;
	query: string;
	page: number;
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
 * Reads the URL into the filter state, correcting what it cannot use and reporting each correction.
 */
export function parseUsageSearch(params: URLSearchParams): UsageSearch {
	const notices: string[] = [];

	const perPage = params.get('per_page');
	if (perPage !== null && perPage !== String(USAGE_RECORDS_PAGE_SIZE)) {
		notices.push(
			`This screen reads ${USAGE_RECORDS_PAGE_SIZE} records per page, so per_page was ignored.`
		);
	}

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
			schemaUsageGroupBy,
			'',
			'group_by value',
			'no breakdown',
			notices
		),
		status: pick(params.get('status'), schemaRequestStatus, '', 'status', 'any status', notices),
		endpointId: pickText(params.get('endpoint_id'), notices),
		model: pickText(params.get('model'), notices),
		query: pickText(params.get('q'), notices),
		page: pick(params.get('page'), pageNumber, 1, 'page', 'page 1', notices),
		notices
	};
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
		search.status !== '' || search.endpointId !== '' || search.model !== '' || search.query !== ''
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
