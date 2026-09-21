// Reading the upstream endpoints tab's filters out of the URL (docs/SPEC-UI/001-SPEC-UI.md §6.2, §8.4.2).
//
// The URL is the source of truth for these filters, which is what makes a filtered view shareable and what
// makes back or forward navigation restore it. That puts two obligations here: a hand-edited URL has to
// resolve into a usable query rather than a failed request, and a correction has to be visible rather than
// silent (§7.1.1).
//
// `provider_id`, `status`, and `page` are the names the API's own list route reads (SPEC-API §7.5), so the
// URL and the request say the same thing and the panel never translates one vocabulary into another.
//
// The provider filter's option list is deliberately not built here. It comes from an unfiltered read of the
// same route, because a list derived from the filtered result would strand the operator on the provider they
// picked: every other provider would vanish from the select until the filter was cleared.

import { z } from 'zod';
import { pageNumber, searchText } from './primitives';
import { ENDPOINT_STATUS_ACTIVE, ENDPOINT_STATUS_DISABLED } from './endpoint';

/** The page size the upstream table reads. §4 offers 25, 50, and 100; this screen reads 25. */
export const ENDPOINTS_PAGE_SIZE = 25;

/**
 * The two states the status filter offers.
 *
 * §6.2 names the filter without naming its members, and these are the two an endpoint's own `status` field
 * carries, which is the set the update form writes. A third value in the URL is one the select cannot show,
 * so it is corrected with a notice rather than sent on as a filter the operator cannot see.
 */
export const ENDPOINT_FILTER_STATUSES = [ENDPOINT_STATUS_ACTIVE, ENDPOINT_STATUS_DISABLED] as const;

export const schemaEndpointFilterStatus = z.enum(ENDPOINT_FILTER_STATUSES);

export type EndpointFilterStatus = (typeof ENDPOINT_FILTER_STATUSES)[number];

/** The filter state the upstream tab reads out of the URL. */
export type EndpointSearch = {
	providerId: string;
	status: EndpointFilterStatus | '';
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

/** Reads the URL into the filter state, correcting what it cannot use and reporting each correction. */
export function parseEndpointSearch(params: URLSearchParams): EndpointSearch {
	const notices: string[] = [];

	const perPage = params.get('per_page');
	if (perPage !== null && perPage !== String(ENDPOINTS_PAGE_SIZE)) {
		notices.push(
			`This screen reads ${ENDPOINTS_PAGE_SIZE} endpoints per page, so per_page was ignored.`
		);
	}

	return {
		providerId: pickText(params.get('provider_id'), notices),
		status: pick(
			params.get('status'),
			schemaEndpointFilterStatus,
			'',
			'status',
			'any status',
			notices
		),
		page: pick(params.get('page'), pageNumber, 1, 'page', 'page 1', notices),
		notices
	};
}

/**
 * True when either filter is set.
 *
 * The page is excluded on purpose: it is not a filter, so counting it would make "Clear filters" appear on
 * a screen where nothing is filtered, and a control that appears to do something when there is nothing to
 * clear is the kind of control R-26 forbids.
 */
export function endpointFiltersApplied(search: EndpointSearch): boolean {
	return search.providerId !== '' || search.status !== '';
}

/**
 * The next URL after a filter changes.
 *
 * A filter change clears the page, because staying on page 7 of a narrower result would show an empty
 * table for a filter that did match rows. Changing the page itself does not, which is the one case where
 * the operator asked to move.
 */
export function nextEndpointSearch(
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
