// Usage schema tests (docs/SPEC-UI/001-SPEC-UI.md §7.6, docs/SPEC-API/001-SPEC-API.md §7.12).
//
// The response shapes are where the API's money and rate rules live, so the cases below are chosen around
// what would silently render wrong rather than around what would fail loudly: a fraction displayed as a
// rate, a null collection iterated as an array, a token count that arrives negative, and an enum member
// the panel has never seen. Each is a case the panel would otherwise show as a plausible number.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { schemaUsageSummary, schemaUsageTimeseries } from '$lib/schemas/usage';

function totals(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		requests: 120,
		tokens_in: 4000,
		tokens_out: 1500,
		tokens_cache_read: 200,
		tokens_cache_write: 100,
		cost_usd: '0.0042',
		latency_ms: 2400,
		latency_p50_ms: 210,
		latency_p95_ms: 880,
		error_count: 6,
		error_rate: '0.0500',
		...overrides
	};
}

function summary(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		from: '2026-09-17T00:00:00Z',
		to: '2026-09-18T00:00:00Z',
		group_by: 'provider',
		totals: totals(),
		groups: [{ key: 'openai', totals: totals({ requests: 100 }) }],
		...overrides
	};
}

function bucket(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return { bucket: '2026-09-17T14:00:00Z', totals: totals(), ...overrides };
}

describe('usage summary', () => {
	it('parses a full summary with its group breakdown', () => {
		const parsed = schemaUsageSummary.safeParse(summary());

		expect(parsed.success).toBe(true);
		expect(parsed.data?.groups[0].key).toBe('openai');
	});

	forEachCase(
		[
			{ name: 'reads a null group list as no breakdown', groups: null, expected: 0 },
			{ name: 'reads an empty group list as no breakdown', groups: [], expected: 0 },
			{ name: 'keeps a single group', groups: [{ key: 'x', totals: totals() }], expected: 1 }
		],
		(testCase) => {
			const parsed = schemaUsageSummary.safeParse(summary({ groups: testCase.groups }));

			expect(parsed.success).toBe(true);
			expect(parsed.data?.groups).toHaveLength(testCase.expected);
		}
	);

	forEachCase(
		[
			{ name: 'accepts a zero cost', cost: '0', ok: true },
			{ name: 'accepts a fractional cost', cost: '0.000001', ok: true },
			{ name: 'rejects a cost with a sign', cost: '-1.00', ok: false },
			{ name: 'rejects a cost in exponent form', cost: '1e-4', ok: false },
			{ name: 'rejects a non-numeric cost', cost: 'free', ok: false }
		],
		(testCase) => {
			const parsed = schemaUsageSummary.safeParse(
				summary({ totals: totals({ cost_usd: testCase.cost }) })
			);

			expect(parsed.success, `${testCase.cost} should ${testCase.ok ? 'parse' : 'fail'}`).toBe(
				testCase.ok
			);
		}
	);

	forEachCase(
		[
			{ name: 'accepts a four-decimal fraction', rate: '0.1250', ok: true },
			{ name: 'accepts one', rate: '1.0000', ok: true },
			{ name: 'accepts zero', rate: '0.0000', ok: true },
			{ name: 'rejects a percentage sign', rate: '12%', ok: false },
			{ name: 'rejects an empty rate', rate: '', ok: false }
		],
		(testCase) => {
			const parsed = schemaUsageSummary.safeParse(
				summary({ totals: totals({ error_rate: testCase.rate }) })
			);

			expect(parsed.success, `${testCase.rate} should ${testCase.ok ? 'parse' : 'fail'}`).toBe(
				testCase.ok
			);
		}
	);

	forEachCase(
		[
			{ name: 'accepts a zero token count', tokens: 0, ok: true },
			{ name: 'accepts a large token count', tokens: 9_007_199_254_740_991, ok: true },
			{ name: 'rejects a negative token count', tokens: -1, ok: false },
			{ name: 'rejects a fractional token count', tokens: 1.5, ok: false }
		],
		(testCase) => {
			const parsed = schemaUsageSummary.safeParse(
				summary({ totals: totals({ tokens_in: testCase.tokens }) })
			);

			expect(parsed.success, `${testCase.tokens} should ${testCase.ok ? 'parse' : 'fail'}`).toBe(
				testCase.ok
			);
		}
	);

	it('rejects a timestamp that is not RFC3339', () => {
		expect(schemaUsageSummary.safeParse(summary({ from: 'yesterday' })).success).toBe(false);
	});

	it('reports a renamed field instead of rendering undefined', () => {
		const { from: _dropped, ...withoutFrom } = summary();

		expect(schemaUsageSummary.safeParse(withoutFrom).success).toBe(false);
	});

	it('tolerates a field the panel does not know yet', () => {
		const parsed = schemaUsageSummary.safeParse(summary({ unknown_future_field: 'value' }));

		expect(parsed.success).toBe(true);
	});
});

describe('usage timeseries', () => {
	it('parses buckets with their totals', () => {
		const parsed = schemaUsageTimeseries.safeParse({
			granularity: 'hour',
			from: '2026-09-17T00:00:00Z',
			to: '2026-09-18T00:00:00Z',
			buckets: [bucket(), bucket({ bucket: '2026-09-17T15:00:00Z' })]
		});

		expect(parsed.success).toBe(true);
		expect(parsed.data?.buckets).toHaveLength(2);
	});

	it('reads a null bucket list as no buckets', () => {
		const parsed = schemaUsageTimeseries.safeParse({
			granularity: 'day',
			from: '2026-09-17T00:00:00Z',
			to: '2026-09-18T00:00:00Z',
			buckets: null
		});

		expect(parsed.data?.buckets).toEqual([]);
	});

	it('rejects a granularity the API does not offer', () => {
		expect(
			schemaUsageTimeseries.safeParse({
				granularity: 'minute',
				from: '2026-09-17T00:00:00Z',
				to: '2026-09-18T00:00:00Z',
				buckets: []
			}).success
		).toBe(false);
	});
});
