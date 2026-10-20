// The Logs filter bar's URL state (docs/SPEC-UI/001-SPEC-UI.md §8.4.2, §7.1.1).
//
// The URL is the filter, so a hand-edited address has to correct itself and say so rather than fail, and
// the answer to "is anything filtered" is what decides whether a Clear control exists at all.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { logFiltersApplied, nextLogSearch, parseLogSearch } from '$lib/schemas/log-search';

describe('parseLogSearch', () => {
	it('defaults to the last 24 hours and the first page when the URL chooses nothing', () => {
		const search = parseLogSearch(new URLSearchParams(''));

		expect(search.period).toBe('24h');
		expect(search.page).toBe(1);
		expect(search.status).toBe('');
		expect(search.endpointId).toBe('');
		expect(search.model).toBe('');
		expect(search.query).toBe('');
		expect(search.notices).toEqual([]);
	});

	it('reads every filter the URL carries', () => {
		const search = parseLogSearch(
			new URLSearchParams('period=7d&status=error&endpoint_id=ep_9&model=gpt-4o&q=timeout&page=2')
		);

		expect(search.period).toBe('7d');
		expect(search.status).toBe('error');
		expect(search.endpointId).toBe('ep_9');
		expect(search.model).toBe('gpt-4o');
		expect(search.query).toBe('timeout');
		expect(search.page).toBe(2);
	});

	it('corrects an unusable period and says so', () => {
		const search = parseLogSearch(new URLSearchParams('period=fortnight'));

		expect(search.period).toBe('24h');
		expect(search.notices.some((notice) => notice.includes('period'))).toBe(true);
	});

	it('corrects an unusable status and says so', () => {
		const search = parseLogSearch(new URLSearchParams('status=partly'));

		expect(search.status).toBe('');
		expect(search.notices.length).toBeGreaterThan(0);
	});

	it('drops a free-text filter longer than 200 characters', () => {
		const search = parseLogSearch(new URLSearchParams(`q=${'x'.repeat(201)}`));

		expect(search.query).toBe('');
		expect(search.notices.length).toBeGreaterThan(0);
	});

	it('ignores a per_page the screen does not read, and says so', () => {
		const search = parseLogSearch(new URLSearchParams('per_page=500'));

		expect(search.notices.some((notice) => notice.includes('per_page'))).toBe(true);
	});
});

describe('logFiltersApplied', () => {
	const base = {
		period: '24h' as const,
		status: '' as const,
		endpointId: '',
		model: '',
		query: '',
		page: 1,
		notices: []
	};

	const cases = [
		{ name: 'is false with no filters set', input: base, expected: false },
		{ name: 'is true for a status', input: { ...base, status: 'error' as const }, expected: true },
		{ name: 'is true for an endpoint id', input: { ...base, endpointId: 'ep_1' }, expected: true },
		{ name: 'is true for a model', input: { ...base, model: 'gpt-4o' }, expected: true },
		{ name: 'is true for a search term', input: { ...base, query: 'timeout' }, expected: true },
		// The period always has a value, so counting it would make "Clear filters" appear where nothing is
		// filtered, which is a control R-26 forbids.
		{
			name: 'is false when only the period is set',
			input: { ...base, period: '30d' as const },
			expected: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(logFiltersApplied(testCase.input), testCase.name).toBe(testCase.expected);
	});
});

describe('nextLogSearch', () => {
	it('sets a filter and clears the page, because a narrower result starts at its first page', () => {
		const next = nextLogSearch(new URLSearchParams('page=7'), 'status', 'error');

		expect(next.get('status')).toBe('error');
		expect(next.get('page')).toBeNull();
	});

	it('keeps the page when the page itself moved', () => {
		const next = nextLogSearch(new URLSearchParams('page=2'), 'page', '3');

		expect(next.get('page')).toBe('3');
	});

	it('removes a filter set to empty rather than sending an empty parameter', () => {
		const next = nextLogSearch(new URLSearchParams('status=error&page=2'), 'status', '');

		expect(next.get('status')).toBeNull();
		expect(next.get('page')).toBeNull();
	});

	it('leaves the other filters intact', () => {
		const next = nextLogSearch(
			new URLSearchParams('period=7d&model=gpt-4o&q=timeout'),
			'status',
			'error'
		);

		expect(next.get('period')).toBe('7d');
		expect(next.get('model')).toBe('gpt-4o');
		expect(next.get('q')).toBe('timeout');
	});
});
