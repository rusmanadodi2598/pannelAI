// Usage schema tests (docs/SPEC-UI/001-SPEC-UI.md §7.6, docs/SPEC-API/001-SPEC-API.md §7.12).
//
// The response shapes are where the API's money and rate rules live, so the cases below are chosen around
// what would silently render wrong rather than around what would fail loudly: a fraction displayed as a
// rate, a null collection iterated as an array, a token count that arrives negative, and an enum member
// the panel has never seen. Each is a case the panel would otherwise show as a plausible number.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	schemaUsageRecordDetail,
	schemaUsageRecordList,
	schemaUsageSummary,
	schemaUsageTimeseries
} from '$lib/schemas/usage';
import { usageQuery } from '$lib/schemas/usage-view';

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

function record(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'usr_1',
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
		endpoint_id: 'ep_1',
		provider_id: 'openai',
		gateway_key_id: 'gky_1',
		model: 'gpt-4o',
		tokens_in: 100,
		tokens_out: 40,
		tokens_cache_read: 0,
		tokens_cache_write: 0,
		cost_usd: '0.0011',
		latency_ms: 320,
		status: 'success',
		...overrides
	};
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

describe('usage records', () => {
	it('parses a page of records with its meta block', () => {
		const parsed = schemaUsageRecordList.safeParse({
			data: [record()],
			meta: { page: 1, per_page: 25, total: 1 }
		});

		expect(parsed.success).toBe(true);
		expect(parsed.data?.meta.total).toBe(1);
	});

	it('reads a null record list as no records', () => {
		const parsed = schemaUsageRecordList.safeParse({
			data: null,
			meta: { page: 1, per_page: 25, total: 0 }
		});

		expect(parsed.data?.data).toEqual([]);
	});

	it('rejects a fractional page size, which is a contract bug worth seeing', () => {
		expect(
			schemaUsageRecordList.safeParse({
				data: [],
				meta: { page: 1, per_page: 25.5, total: 0 }
			}).success
		).toBe(false);
	});

	forEachCase(
		[
			{ name: 'keeps the success status', status: 'success', ok: true },
			{ name: 'keeps the error status', status: 'error', ok: true },
			{ name: 'rejects a status the panel does not know', status: 'timeout', ok: false }
		],
		(testCase) => {
			const parsed = schemaUsageRecordList.safeParse({
				data: [record({ status: testCase.status })],
				meta: { page: 1, per_page: 25, total: 1 }
			});

			expect(parsed.success, `${testCase.status} should ${testCase.ok ? 'parse' : 'fail'}`).toBe(
				testCase.ok
			);
		}
	);

	it('keeps the optional identifiers absent rather than blank', () => {
		const parsed = schemaUsageRecordList.safeParse({
			data: [
				record({
					endpoint_id: undefined,
					gateway_key_id: undefined,
					error_code: 'UPSTREAM_TIMEOUT'
				})
			],
			meta: { page: 1, per_page: 25, total: 1 }
		});

		expect(parsed.data?.data[0].endpoint_id).toBeUndefined();
		expect(parsed.data?.data[0].error_code).toBe('UPSTREAM_TIMEOUT');
	});
});

describe('usage record detail', () => {
	it('carries the record and says capture is off without a log', () => {
		const parsed = schemaUsageRecordDetail.safeParse({
			usage: record(),
			capture_enabled: false
		});

		expect(parsed.success).toBe(true);
		expect(parsed.data?.log).toBeUndefined();
	});

	it('carries the captured log when capture is on', () => {
		const parsed = schemaUsageRecordDetail.safeParse({
			usage: record(),
			capture_enabled: true,
			log: {
				request_id: 'req_1',
				ts: '2026-09-17T14:03:00Z',
				status: 'success',
				latency_ms: 320,
				capture_enabled: true,
				capture_body_max_bytes: 65536,
				request_body: '{"model":"gpt-4o"}',
				response_body: '{"ok":true}'
			}
		});

		expect(parsed.success).toBe(true);
		expect(parsed.data?.log?.request_body).toBe('{"model":"gpt-4o"}');
	});

	it('requires the capture flag, because its absence would render as "off" without the API saying so', () => {
		expect(schemaUsageRecordDetail.safeParse({ usage: record() }).success).toBe(false);
	});
});

describe('usage query', () => {
	const range = { from: '2026-09-17T00:00:00Z', to: '2026-09-18T00:00:00Z' };

	it('always sends the range, even with every filter unset', () => {
		expect(usageQuery(range)).toEqual(range);
	});

	it('sends every filter when they are all set', () => {
		expect(
			usageQuery({
				...range,
				groupBy: 'gateway_key',
				granularity: 'day',
				status: 'error',
				endpointId: 'ep_1',
				model: 'gpt-4o',
				query: 'timeout',
				page: 3
			})
		).toEqual({
			...range,
			group_by: 'gateway_key',
			granularity: 'day',
			status: 'error',
			endpoint_id: 'ep_1',
			model: 'gpt-4o',
			q: 'timeout',
			page: 3
		});
	});

	forEachCase(
		[
			{ name: 'drops an empty group_by rather than sending it', field: 'groupBy', value: '' },
			{ name: 'drops a whitespace-only group_by', field: 'groupBy', value: '   ' },
			{ name: 'drops an empty model filter', field: 'model', value: '' },
			{ name: 'drops a whitespace-only search term', field: 'query', value: ' \t ' }
		],
		(testCase) => {
			const query = usageQuery({ ...range, [testCase.field]: testCase.value });
			const key = { groupBy: 'group_by', model: 'model', query: 'q' }[testCase.field] ?? '';

			expect(Object.keys(query)).toEqual(['from', 'to']);
			expect(query[key as 'group_by']).toBeUndefined();
		}
	);

	it('trims a filter instead of sending the padded value', () => {
		expect(usageQuery({ ...range, model: '  gpt-4o  ' }).model).toBe('gpt-4o');
	});

	it('omits the page when the caller did not ask for one', () => {
		expect(usageQuery(range).page).toBeUndefined();
	});
});
