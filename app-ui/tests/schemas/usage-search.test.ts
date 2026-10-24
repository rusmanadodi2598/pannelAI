// The Usage screen's URL state (docs/SPEC-UI/001-SPEC-UI.md §6.5, §7.6).
//
// Split from `usage-view.test.ts`, which maps the operator's choices onto an API query; this maps the same
// choices onto the address bar. The two directions have to agree, because a shared link is the only way one
// operator shows another what they are looking at: a filter parsed back as something it was not, or a page
// that resets when only the period changed, both make the link lie.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { nextUsageSearch, parseUsageSearch, usageFiltersApplied } from '$lib/schemas/usage-search';

describe('parseUsageSearch', () => {
	it('falls back to the API default window when the URL says nothing', () => {
		const search = parseUsageSearch(new URLSearchParams());

		expect(search).toEqual({
			period: '24h',
			groupBy: 'model',
			sort: '',
			order: 'asc',
			status: '',
			endpointId: '',
			providerId: '',
			gatewayKeyId: '',
			model: '',
			query: '',
			page: 1,
			perPageRequested: 25,
			perPageNotice: null,
			notices: []
		});
	});

	it('reads every filter the Records tab writes', () => {
		const search = parseUsageSearch(
			new URLSearchParams(
				'period=7d&group_by=model&status=error&endpoint_id=ep_1&provider_id=openai&gateway_key_id=gk_1&model=gpt-4o&q=timeout&page=3&sort=cost_usd&order=desc'
			)
		);

		expect(search).toMatchObject({
			period: '7d',
			groupBy: 'model',
			sort: 'cost_usd',
			order: 'desc',
			status: 'error',
			endpointId: 'ep_1',
			providerId: 'openai',
			gatewayKeyId: 'gk_1',
			model: 'gpt-4o',
			query: 'timeout',
			page: 3,
			perPageRequested: 25,
			perPageNotice: null,
			notices: []
		});
	});

	it('reads `none` as the operator turning the breakdown off', () => {
		const search = parseUsageSearch(new URLSearchParams('group_by=none'));

		expect(search.groupBy).toBe('none');
		expect(search.notices).toEqual([]);
	});

	forEachCase(
		[
			{ name: 'rejects a period the panel does not offer', query: 'period=forever', notices: 1 },
			{ name: 'rejects a group_by the API would reject', query: 'group_by=nonsense', notices: 1 },
			{ name: 'rejects a sort the panel does not offer', query: 'sort=vibes', notices: 1 },
			{ name: 'rejects an order the panel does not offer', query: 'order=sideways', notices: 1 },
			{ name: 'rejects a status the API does not store', query: 'status=pending', notices: 1 },
			{ name: 'rejects a page below the first', query: 'page=0', notices: 1 },
			{ name: 'rejects a page that is not a number', query: 'page=last', notices: 1 },
			{
				name: 'reports each unusable value rather than only the first',
				query: 'period=forever&status=pending&page=0',
				notices: 3
			}
		],
		(testCase) => {
			const search = parseUsageSearch(new URLSearchParams(testCase.query));

			expect(search.notices).toHaveLength(testCase.notices);
			// Every notice names the parameter, so the operator can find it in the URL (§8.10, R-27).
			for (const notice of search.notices) {
				expect(notice).toMatch(/period|group_by|sort|order|status|page|filter/);
			}
		}
	);

	it('corrects an unusable period to the default rather than showing nothing', () => {
		const search = parseUsageSearch(new URLSearchParams('period=forever'));

		expect(search.period).toBe('24h');
		expect(search.notices[0]).toContain('forever');
	});

	it('drops a filter longer than the API accepts instead of sending it', () => {
		const search = parseUsageSearch(new URLSearchParams(`q=${'x'.repeat(300)}`));

		expect(search.query).toBe('');
		expect(search.notices).toHaveLength(1);
	});

	it('keeps a filter at the 200 character bound', () => {
		const search = parseUsageSearch(new URLSearchParams(`q=${'x'.repeat(200)}`));

		expect(search.query).toHaveLength(200);
		expect(search.notices).toEqual([]);
	});

	it('ignores a query parameter the panel does not read', () => {
		// A foreign parameter is not the panel's business, and failing the screen over one would make the
		// URL unusable for anything the panel does not know about.
		expect(parseUsageSearch(new URLSearchParams('utm_source=chat')).notices).toEqual([]);
	});

	it('collapses whitespace in a search term', () => {
		expect(parseUsageSearch(new URLSearchParams('q=timeout+++upstream')).query).toBe(
			'timeout upstream'
		);
	});
});

describe('usageFiltersApplied', () => {
	const base = parseUsageSearch(new URLSearchParams());

	forEachCase(
		[
			{ name: 'is false when only the period is set', query: 'period=30d', expected: false },
			{ name: 'is true once a status is chosen', query: 'status=error', expected: true },
			{ name: 'is true once an endpoint is named', query: 'endpoint_id=ep_1', expected: true },
			{ name: 'is true once a provider is chosen', query: 'provider_id=openai', expected: true },
			{
				name: 'is true once a gateway key is chosen',
				query: 'gateway_key_id=gk_1',
				expected: true
			},
			{ name: 'is true once a model is named', query: 'model=gpt-4o', expected: true },
			{ name: 'is true once a search term is entered', query: 'q=timeout', expected: true },
			{
				name: 'stays false for a page change, which is not a filter',
				query: 'page=4',
				expected: false
			}
		],
		(testCase) => {
			const search = parseUsageSearch(new URLSearchParams(testCase.query));

			expect(usageFiltersApplied(search)).toBe(testCase.expected);
		}
	);

	it('is false for the defaults, so the Clear control is not offered when there is nothing to clear', () => {
		expect(usageFiltersApplied(base)).toBe(false);
	});
});

describe('nextUsageSearch', () => {
	it('sets a filter', () => {
		expect(nextUsageSearch(new URLSearchParams(), 'period', '7d').toString()).toBe('period=7d');
	});

	it('clears a filter when the value is empty', () => {
		expect(nextUsageSearch(new URLSearchParams('status=error'), 'status', '').toString()).toBe('');
	});

	it('returns to the first page when a filter changes', () => {
		// Staying on page 7 of a narrower result would show an empty table for a filter that matched rows.
		expect(
			nextUsageSearch(new URLSearchParams('page=7&model=gpt-4o'), 'status', 'error').get('page')
		).toBeNull();
	});

	it('keeps the page when the page is what changed', () => {
		expect(nextUsageSearch(new URLSearchParams('page=7'), 'page', '8').get('page')).toBe('8');
	});

	it('leaves the other filters alone', () => {
		const next = nextUsageSearch(new URLSearchParams('period=7d&model=gpt-4o'), 'status', 'error');

		expect(next.get('period')).toBe('7d');
		expect(next.get('model')).toBe('gpt-4o');
		expect(next.get('status')).toBe('error');
	});

	it('does not mutate the parameters it was handed', () => {
		const current = new URLSearchParams('period=7d');

		nextUsageSearch(current, 'period', '30d');

		expect(current.get('period')).toBe('7d');
	});
});
