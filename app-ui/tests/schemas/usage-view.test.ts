// Usage view derivation tests (docs/SPEC-UI/001-SPEC-UI.md §6.5, §7.6).
//
// These are the mappings between what the operator chooses and what the API is asked, plus the two
// figures the panel derives for display. They are pure functions, so the cases run without a clock, a
// request, or a DOM, and the boundaries that matter are the ones a screen would otherwise get subtly
// wrong: a period that starts at a UTC day boundary rather than 24 hours ago, a filter that must not be
// sent when it is empty, and a count that rounds to zero without being zero.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	barHeights,
	errorRatePercent,
	formatCount,
	granularityFor,
	periodRange,
	seriesPeak,
	seriesSummary,
	seriesTotal,
	type SeriesPoint
} from '$lib/schemas/usage-view';
import { nextUsageSearch, parseUsageSearch, usageFiltersApplied } from '$lib/schemas/usage-search';
import { USAGE_PERIODS } from '$lib/schemas/usage';

const NOW = new Date('2026-09-18T14:23:45.678Z');

describe('periodRange', () => {
	forEachCase(
		[
			{
				name: 'starts today at the UTC calendar boundary, not 24 hours ago',
				now: NOW,
				period: 'today' as const,
				from: '2026-09-18T00:00:00Z',
				to: '2026-09-18T14:23:45Z'
			},
			{
				name: 'gives a midnight start an empty range rather than an inverted one',
				now: new Date('2026-09-18T00:00:00.000Z'),
				period: 'today' as const,
				from: '2026-09-18T00:00:00Z',
				to: '2026-09-18T00:00:00Z'
			},
			{
				name: 'keeps the last second of the UTC day inside today',
				now: new Date('2026-09-18T23:59:59.999Z'),
				period: 'today' as const,
				from: '2026-09-18T00:00:00Z',
				to: '2026-09-18T23:59:59Z'
			},
			{
				name: 'crosses a month boundary for a 30 day span',
				now: NOW,
				period: '30d' as const,
				from: '2026-08-19T14:23:45Z',
				to: '2026-09-18T14:23:45Z'
			},
			{
				name: 'crosses a year boundary for a 7 day span',
				now: new Date('2026-01-05T10:00:00Z'),
				period: '7d' as const,
				from: '2025-12-29T10:00:00Z',
				to: '2026-01-05T10:00:00Z'
			},
			{
				name: 'spans 24 hours for the 24h period',
				now: NOW,
				period: '24h' as const,
				from: '2026-09-17T14:23:45Z',
				to: '2026-09-18T14:23:45Z'
			},
			{
				name: 'spans 60 days for the longest period the panel offers',
				now: NOW,
				period: '60d' as const,
				from: '2026-07-20T14:23:45Z',
				to: '2026-09-18T14:23:45Z'
			}
		],
		(testCase) => {
			expect(periodRange(testCase.period, testCase.now)).toEqual({
				from: testCase.from,
				to: testCase.to
			});
		}
	);

	it('never sends milliseconds, so a shared URL is stable across two reads', () => {
		for (const period of USAGE_PERIODS) {
			const range = periodRange(period, NOW);

			expect(range.from, `${period} from`).not.toContain('.');
			expect(range.to, `${period} to`).not.toContain('.');
		}
	});

	it('never returns an inverted range, for any period and any instant', () => {
		const instants = [
			new Date('2026-01-01T00:00:00Z'),
			new Date('2026-06-15T12:30:00Z'),
			new Date('2026-12-31T23:59:59Z')
		];

		for (const period of USAGE_PERIODS) {
			for (const instant of instants) {
				const range = periodRange(period, instant);

				expect(Date.parse(range.from), `${period} at ${instant.toISOString()}`).toBeLessThanOrEqual(
					Date.parse(range.to)
				);
			}
		}
	});
});

describe('granularityFor', () => {
	forEachCase(
		[
			{ name: 'buckets a single day by hour', period: 'today' as const, expected: 'hour' },
			{ name: 'buckets the last 24 hours by hour', period: '24h' as const, expected: 'hour' },
			{
				name: 'buckets a week by day, because 168 hourly bars are not a chart',
				period: '7d' as const,
				expected: 'day'
			},
			{ name: 'buckets 30 days by day', period: '30d' as const, expected: 'day' },
			{ name: 'buckets 60 days by day', period: '60d' as const, expected: 'day' }
		],
		(testCase) => {
			expect(granularityFor(testCase.period)).toBe(testCase.expected);
		}
	);

	it('offers only granularities the API accepts', () => {
		for (const period of USAGE_PERIODS) {
			expect(['hour', 'day']).toContain(granularityFor(period));
		}
	});
});

describe('formatCount', () => {
	forEachCase(
		[
			{ name: 'formats zero', value: 0, expected: '0' },
			{ name: 'leaves three digits ungrouped', value: 999, expected: '999' },
			{ name: 'groups a thousand', value: 1000, expected: '1,000' },
			{ name: 'groups a million', value: 1234567, expected: '1,234,567' },
			{
				name: 'groups the largest exact integer',
				value: 9007199254740991,
				expected: '9,007,199,254,740,991'
			}
		],
		(testCase) => {
			expect(formatCount(testCase.value)).toBe(testCase.expected);
		}
	);
});

describe('errorRatePercent', () => {
	forEachCase(
		[
			{ name: 'renders an empty window as 0.00%', rate: '0.0000', expected: '0.00%' },
			{ name: 'keeps the smallest non-zero rate visible', rate: '0.0001', expected: '0.01%' },
			{ name: 'renders a twentieth as 5.00%', rate: '0.0500', expected: '5.00%' },
			{ name: 'renders an eighth as 12.50%', rate: '0.1250', expected: '12.50%' },
			{ name: 'renders a third as 33.33%', rate: '0.3333', expected: '33.33%' },
			{ name: 'renders every request failing as 100.00%', rate: '1.0000', expected: '100.00%' }
		],
		(testCase) => {
			expect(errorRatePercent(testCase.rate)).toBe(testCase.expected);
		}
	);

	it('never rounds a four-decimal fraction away, which is why it is not a float display', () => {
		// The API formats the rate with four decimal places, so one unit of the last place is 0.01% here.
		for (let last = 0; last <= 9; last += 1) {
			const rate = `0.000${last}`;

			expect(errorRatePercent(rate)).toBe(`0.0${last}%`);
		}
	});
});

describe('seriesPeak and seriesTotal', () => {
	function points(values: number[]): SeriesPoint[] {
		return values.map((value, index) => ({ bucket: `bucket-${index}`, value }));
	}

	it('has no peak and a zero total for an empty series', () => {
		expect(seriesPeak([])).toBeNull();
		expect(seriesTotal([])).toBe(0);
	});

	forEachCase(
		[
			{ name: 'finds the only point', values: [7], expected: 'bucket-0' },
			{ name: 'finds a clear peak in the middle', values: [1, 9, 2], expected: 'bucket-1' },
			{
				name: 'takes the first of equal peaks, so the summary is stable',
				values: [5, 5, 1],
				expected: 'bucket-0'
			},
			{
				name: 'reports a peak of zero for an all-zero series',
				values: [0, 0, 0],
				expected: 'bucket-0'
			}
		],
		(testCase) => {
			expect(seriesPeak(points(testCase.values))?.bucket).toBe(testCase.expected);
		}
	);

	forEachCase(
		[
			{ name: 'totals a single point', values: [7], expected: 7 },
			{ name: 'totals across points', values: [1, 2, 3], expected: 6 },
			{ name: 'totals an all-zero series', values: [0, 0], expected: 0 },
			{ name: 'totals large counts exactly', values: [1_000_000, 2_000_000], expected: 3_000_000 }
		],
		(testCase) => {
			expect(seriesTotal(points(testCase.values))).toBe(testCase.expected);
		}
	);
});

describe('seriesSummary', () => {
	const label = (bucket: string): string => bucket;

	function points(values: number[]): SeriesPoint[] {
		return values.map((value, index) => ({ bucket: `b${index}`, value }));
	}

	it('says the window is empty rather than showing a zero', () => {
		expect(seriesSummary([], label, 'requests')).toBe('No requests in this window.');
	});

	it('describes a single bucket without claiming a peak among one', () => {
		expect(seriesSummary(points([12]), label, 'requests')).toBe('One bucket at b0, 12 requests.');
	});

	it('names the bucket count, the peak, and the total', () => {
		expect(seriesSummary(points([1, 9, 2]), label, 'requests')).toBe(
			'3 buckets. Highest 9 at b1; 12 requests in total.'
		);
	});

	it('groups the numbers it prints, so a long series is still readable', () => {
		expect(seriesSummary(points([1200, 3400]), label, 'tokens')).toBe(
			'2 buckets. Highest 3,400 at b1; 4,600 tokens in total.'
		);
	});
});

describe('barHeights', () => {
	forEachCase(
		[
			{ name: 'has no bars for an empty series', values: [], expected: [] },
			{ name: 'gives the tallest bar the full height', values: [3], expected: [100] },
			{ name: 'scales the rest against the peak', values: [1, 3], expected: [33, 100] },
			{ name: 'leaves a zero bucket with no bar', values: [0, 3], expected: [0, 100] },
			{ name: 'gives an all-zero series no bars at all', values: [0, 0], expected: [0, 0] },
			{
				name: 'keeps a tiny non-zero bucket visible instead of drawing nothing',
				values: [1, 1000],
				expected: [1, 100]
			}
		],
		(testCase) => {
			expect(
				barHeights(testCase.values.map((value, index) => ({ bucket: `b${index}`, value })))
			).toEqual(testCase.expected);
		}
	);
});

describe('parseUsageSearch', () => {
	it('falls back to the API default window when the URL says nothing', () => {
		const search = parseUsageSearch(new URLSearchParams());

		expect(search).toEqual({
			period: '24h',
			groupBy: '',
			status: '',
			endpointId: '',
			model: '',
			query: '',
			page: 1,
			notices: []
		});
	});

	it('reads every filter the Records tab writes', () => {
		const search = parseUsageSearch(
			new URLSearchParams(
				'period=7d&group_by=model&status=error&endpoint_id=ep_1&model=gpt-4o&q=timeout&page=3'
			)
		);

		expect(search).toMatchObject({
			period: '7d',
			groupBy: 'model',
			status: 'error',
			endpointId: 'ep_1',
			model: 'gpt-4o',
			query: 'timeout',
			page: 3,
			notices: []
		});
	});

	forEachCase(
		[
			{ name: 'rejects a period the panel does not offer', query: 'period=forever', notices: 1 },
			{ name: 'rejects a group_by the API would reject', query: 'group_by=nonsense', notices: 1 },
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
				expect(notice).toMatch(/period|group_by|status|page|filter/);
			}
		}
	);

	it('corrects an unusable period to the default rather than showing nothing', () => {
		const search = parseUsageSearch(new URLSearchParams('period=forever'));

		expect(search.period).toBe('24h');
		expect(search.notices[0]).toContain('forever');
	});

	forEachCase(
		[
			{ name: 'accepts the page size the screen reads', query: 'per_page=25', notices: 0 },
			{ name: 'refuses to widen the page size from the URL', query: 'per_page=100000', notices: 1 },
			{ name: 'reports a page size that is not a number', query: 'per_page=lots', notices: 1 }
		],
		(testCase) => {
			expect(parseUsageSearch(new URLSearchParams(testCase.query)).notices).toHaveLength(
				testCase.notices
			);
		}
	);

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
