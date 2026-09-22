// Usage breakdown derivations: the panel's own ordering, and the money it prints.
//
// The API sends the whole group block and every cost as a decimal string, so these two mappings decide
// what the operator reads: a table ordered by a column the API never ranked, and a figure that keeps the
// precision the API sent instead of rounding to cents. They are pure functions, so the cases run without
// a clock, a request, or a DOM.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { formatCost, seriesSummary, sortGroups, type SeriesPoint } from '$lib/schemas/usage-view';

describe('sortGroups', () => {
	// The API sends the whole breakdown, so ordering it is a view and not a read: these cases exist because
	// a table whose order is wrong reads as a ranking the API never made.
	function group(key: string, overrides: Record<string, unknown> = {}) {
		return {
			key,
			totals: {
				requests: 10,
				tokens_in: 100,
				tokens_out: 50,
				tokens_cache_read: 5,
				tokens_cache_write: 2,
				cost_usd: '0.0100',
				latency_ms: 10,
				latency_p50_ms: 10,
				latency_p95_ms: 20,
				error_count: 1,
				error_rate: '0.1000',
				...overrides
			}
		};
	}

	const rows = [
		group('openai', { requests: 30, cost_usd: '0.3000' }),
		group('anthropic', { requests: 10, cost_usd: '0.0100' }),
		group('google', { requests: 20, cost_usd: '0.2000' })
	];

	it('keeps the API order when nothing is chosen', () => {
		expect(sortGroups(rows, '', 'asc').map((row) => row.key)).toEqual([
			'openai',
			'anthropic',
			'google'
		]);
	});

	it('orders by the chosen numeric column in the chosen direction', () => {
		expect(sortGroups(rows, 'requests', 'desc').map((row) => row.key)).toEqual([
			'openai',
			'google',
			'anthropic'
		]);
		expect(sortGroups(rows, 'requests', 'asc').map((row) => row.key)).toEqual([
			'anthropic',
			'google',
			'openai'
		]);
	});

	it('compares the dimension by code point, so the order does not move with the locale', () => {
		expect(sortGroups(rows, 'key', 'asc').map((row) => row.key)).toEqual([
			'anthropic',
			'google',
			'openai'
		]);
	});

	it('parses a decimal string column before comparing it', () => {
		// The cost arrives as a string, and comparing those as text would rank "0.3000" below "0.2000".
		expect(sortGroups(rows, 'cost_usd', 'desc').map((row) => row.key)).toEqual([
			'openai',
			'google',
			'anthropic'
		]);
	});

	it('keeps the API order for equal values, so the panel does not invent a rank', () => {
		const tied = [group('first', { requests: 5 }), group('second', { requests: 5 })];

		expect(sortGroups(tied, 'requests', 'desc').map((row) => row.key)).toEqual(['first', 'second']);
	});

	it('does not mutate the rows it was handed', () => {
		const before = rows.map((row) => row.key);

		sortGroups(rows, 'requests', 'desc');

		expect(rows.map((row) => row.key)).toEqual(before);
	});
});

describe('formatCost', () => {
	forEachCase(
		[
			{
				name: 'keeps the API precision rather than rounding to cents',
				value: 0.0042,
				expected: '$0.0042'
			},
			{ name: 'groups a large figure', value: 1234.5, expected: '$1,234.5000' },
			{ name: 'prints a zero cost as a figure, not as nothing', value: 0, expected: '$0.0000' }
		],
		(testCase) => {
			expect(formatCost(testCase.value)).toBe(testCase.expected);
		}
	);
});

describe('seriesSummary with a formatter', () => {
	it('renders a cost series with the currency precision, not with grouped counts', () => {
		const points: SeriesPoint[] = [
			{ bucket: '2026-09-17T13:00:00Z', value: 0.0123 },
			{ bucket: '2026-09-17T14:00:00Z', value: 0.0042 }
		];

		expect(seriesSummary(points, () => 'a bucket', 'USD', formatCost)).toBe(
			'2 buckets. Highest $0.0123 at a bucket; $0.0165 USD in total.'
		);
	});
});
