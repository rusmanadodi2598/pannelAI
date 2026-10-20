// The Usage chart's series arithmetic (docs/SPEC-UI/001-SPEC-UI.md §6.5).
//
// Split with the module: `usage-view.test.ts` proves what the screen asks the API for, this proves what it
// computes from the points that come back. The heights are the interesting half, because a bar of zero is
// not a small bar but an absence, and a window of thousands with one request in it happened.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	barHeights,
	seriesPeak,
	seriesSummary,
	seriesTotal,
	type SeriesPoint
} from '$lib/schemas/usage-series';

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
