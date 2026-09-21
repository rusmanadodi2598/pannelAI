// Endpoint filter reading tests (docs/SPEC-UI/001-SPEC-UI.md §6.2, §8.4.2).
//
// The URL is operator-editable, so the cases below are the ones where a hand-edited address would otherwise
// reach the API as a request the operator did not mean: a status the select cannot show, a page that is not
// a number, a filter value longer than the field's bound. Each correction is asserted with its notice,
// because §7.1.1 makes a silent correction a defect of its own.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	ENDPOINTS_PAGE_SIZE,
	endpointFiltersApplied,
	nextEndpointSearch,
	parseEndpointSearch
} from '$lib/schemas/endpoint-search';

function params(query: string): URLSearchParams {
	return new URLSearchParams(query);
}

describe('parseEndpointSearch', () => {
	it('reads a bare address as the unfiltered first page', () => {
		const search = parseEndpointSearch(params(''));

		expect(search.providerId).toBe('');
		expect(search.status).toBe('');
		expect(search.page).toBe(1);
		expect(search.notices).toEqual([]);
	});

	it('reads every filter the URL carries', () => {
		const search = parseEndpointSearch(params('provider_id=anthropic&status=disabled&page=3'));

		expect(search.providerId).toBe('anthropic');
		expect(search.status).toBe('disabled');
		expect(search.page).toBe(3);
		expect(search.notices).toEqual([]);
	});

	forEachCase(
		[
			{
				name: 'corrects a status the select cannot show',
				query: 'status=broken',
				expected: { status: '', page: 1 },
				noticeIncludes: 'status'
			},
			{
				name: 'corrects a page that is not a number',
				query: 'page=soon',
				expected: { status: '', page: 1 },
				noticeIncludes: 'page'
			},
			{
				name: 'corrects a page below the first one',
				query: 'page=0',
				expected: { status: '', page: 1 },
				noticeIncludes: 'page'
			}
		],
		(testCase) => {
			const search = parseEndpointSearch(params(testCase.query));

			expect(search.status).toBe(testCase.expected.status);
			expect(search.page).toBe(testCase.expected.page);
			expect(search.notices.join(' ')).toContain(testCase.noticeIncludes);
		}
	);

	it('leaves out a filter value longer than the field bound, without echoing it', () => {
		const long = 'p'.repeat(201);
		const search = parseEndpointSearch(params(`provider_id=${long}`));

		expect(search.providerId).toBe('');
		expect(search.notices.join(' ')).toContain('200 characters');
		// The rejected value is not printed back: its length is the reason it failed.
		expect(search.notices.join(' ')).not.toContain(long);
	});

	it('ignores per_page, which is the screen size rather than a filter', () => {
		const search = parseEndpointSearch(params('per_page=50'));

		expect(search.notices.join(' ')).toContain(String(ENDPOINTS_PAGE_SIZE));
	});

	it('accepts the page size this screen reads without a notice', () => {
		const search = parseEndpointSearch(params(`per_page=${ENDPOINTS_PAGE_SIZE}`));

		expect(search.notices).toEqual([]);
	});

	it('ignores a parameter that belongs to another screen', () => {
		const search = parseEndpointSearch(params('provider=openai&q=timeout'));

		expect(search.providerId).toBe('');
		expect(search.notices).toEqual([]);
	});
});

describe('endpointFiltersApplied', () => {
	forEachCase(
		[
			{ name: 'nothing set', query: '', applied: false },
			{ name: 'a provider only', query: 'provider_id=openai', applied: true },
			{ name: 'a status only', query: 'status=active', applied: true },
			{ name: 'both filters', query: 'provider_id=openai&status=active', applied: true },
			// A page is not a filter: counting it would offer "Clear filters" with nothing to clear (R-26).
			{ name: 'a page only', query: 'page=4', applied: false }
		],
		(testCase) => {
			expect(endpointFiltersApplied(parseEndpointSearch(params(testCase.query)))).toBe(
				testCase.applied
			);
		}
	);
});

describe('nextEndpointSearch', () => {
	it('sets a filter and clears the page, because page 3 of a narrower result may be empty', () => {
		const next = nextEndpointSearch(params('provider_id=openai&page=3'), 'status', 'active');

		expect(next.get('provider_id')).toBe('openai');
		expect(next.get('status')).toBe('active');
		expect(next.get('page')).toBeNull();
	});

	it('deletes a filter that was set back to the unfiltered value', () => {
		const next = nextEndpointSearch(params('provider_id=openai&status=active'), 'status', '');

		expect(next.get('status')).toBeNull();
		expect(next.get('provider_id')).toBe('openai');
	});

	it('keeps the page when the page is what changed', () => {
		const next = nextEndpointSearch(params('page=2'), 'page', '3');

		expect(next.get('page')).toBe('3');
	});

	it('keeps a parameter that is not a filter, so the create form is not closed by filtering', () => {
		const next = nextEndpointSearch(params('provider=openai'), 'status', 'active');

		expect(next.get('provider')).toBe('openai');
	});
});
