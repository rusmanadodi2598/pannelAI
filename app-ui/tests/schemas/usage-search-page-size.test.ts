// The page size the Usage URL asks for, and the correction it gets (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F1).
//
// Split from `usage-view.test.ts` when the pass that added it pushed that file further over the line
// ceiling: the screen reads one page size while the API accepts up to 100, so a `per_page` a shared link
// carries is corrected rather than obeyed, and that rule is its own concern.
//
// The table walks the schema's own boundaries as well as values the schema accepts but the screen does not,
// because from the operator's side both are the same correction.

import { describe, expect, it } from 'vitest';
import { cleanedUsageSearch, parseUsageSearch } from '$lib/schemas/usage-search';
import { forEachCase } from '../support/tables';

describe('parseUsageSearch page size', () => {
	forEachCase(
		[
			{
				name: 'accepts the page size the screen reads',
				query: 'per_page=25',
				requested: 25,
				corrected: false
			},
			{
				name: 'corrects a larger page size',
				query: 'per_page=100',
				requested: 100,
				corrected: true
			},
			{
				name: 'corrects the smallest page size',
				query: 'per_page=1',
				requested: 1,
				corrected: true
			},
			{ name: 'corrects a zero page size', query: 'per_page=0', requested: 25, corrected: true },
			{
				name: 'corrects a negative page size',
				query: 'per_page=-5',
				requested: 25,
				corrected: true
			},
			{
				name: 'corrects a fractional page size',
				query: 'per_page=12.5',
				requested: 25,
				corrected: true
			},
			{
				name: 'corrects a page size that is not a number',
				query: 'per_page=lots',
				requested: 25,
				corrected: true
			},
			{
				name: 'corrects a page size beyond the API bound',
				query: 'per_page=100000',
				requested: 25,
				corrected: true
			},
			{
				name: 'reads the first page size when the URL repeats the parameter',
				query: 'per_page=25&per_page=100000',
				requested: 25,
				corrected: false
			},
			{
				name: 'leaves an absent page size alone',
				query: 'page=2',
				requested: 25,
				corrected: false
			},
			{
				name: 'leaves a parameter the screen does not read alone',
				query: 'utm_source=chat',
				requested: 25,
				corrected: false
			}
		],
		(testCase) => {
			const search = parseUsageSearch(new URLSearchParams(testCase.query));

			expect(search.perPageRequested, testCase.query).toBe(testCase.requested);
			expect(search.perPageNotice !== null, testCase.query).toBe(testCase.corrected);
		}
	);

	it('names the page size the screen reads rather than echoing the rejected value', () => {
		const search = parseUsageSearch(new URLSearchParams(`per_page=${'9'.repeat(300)}`));

		expect(search.perPageNotice).toContain('25');
		expect(search.perPageNotice).not.toContain('999');
	});

	it('keeps a corrected page size out of the notice list, because it is not a filter', () => {
		expect(parseUsageSearch(new URLSearchParams('per_page=100000')).notices).toEqual([]);
	});
});

describe('cleanedUsageSearch', () => {
	forEachCase(
		[
			{
				name: 'has nothing to clean when the page size is the one the screen reads',
				query: 'per_page=25',
				cleaned: false
			},
			{
				name: 'has nothing to clean when the URL names no page size',
				query: 'period=7d',
				cleaned: false
			},
			{
				name: 'has nothing to clean for a parameter the screen does not read',
				query: 'utm_source=chat',
				cleaned: false
			},
			{
				name: 'removes a page size the screen cannot use',
				query: 'per_page=100000',
				cleaned: true
			},
			{ name: 'removes a page size that is not a number', query: 'per_page=lots', cleaned: true }
		],
		(testCase) => {
			const cleaned = cleanedUsageSearch(new URLSearchParams(testCase.query));

			expect(cleaned !== null, testCase.query).toBe(testCase.cleaned);
			if (cleaned !== null) expect(cleaned.get('per_page')).toBeNull();
		}
	);

	it('keeps every other parameter, so the correction is not a reset', () => {
		const cleaned = cleanedUsageSearch(
			new URLSearchParams('period=7d&model=gpt-4o&page=3&per_page=100000')
		);

		expect(cleaned?.toString()).toBe('period=7d&model=gpt-4o&page=3');
	});

	it('removes a page size rather than leaving a value the panel would read again', () => {
		const cleaned = cleanedUsageSearch(new URLSearchParams('per_page=100000'));

		expect(cleaned?.toString()).toBe('');
	});

	it('does not mutate the parameters it was handed', () => {
		const current = new URLSearchParams('per_page=100000');

		cleanedUsageSearch(current);

		expect(current.get('per_page')).toBe('100000');
	});
});
