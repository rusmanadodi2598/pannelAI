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
	errorRatePercent,
	formatCount,
	granularityFor,
	periodRange
} from '$lib/schemas/usage-view';
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
