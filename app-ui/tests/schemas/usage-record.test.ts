// The Usage record shapes (docs/SPEC-UI/001-SPEC-UI.md §7.6, docs/SPEC-API/001-SPEC-API.md §7.12).
//
// Split from `usage.test.ts`, which proves the aggregate shapes: a summary's money and rate, a timeseries'
// buckets. This proves what one request's row and detail accept and refuse, which is where the panel's
// records table and its drawer get their facts. The same rule as the file it came from: the cases are chosen
// around what would silently render wrong rather than what would fail loudly.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { schemaUsageRecordDetail, schemaUsageRecordList } from '$lib/schemas/usage';
import { usageQuery } from '$lib/schemas/usage-view';

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
				providerId: 'openai',
				gatewayKeyId: 'gky_1',
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
			provider_id: 'openai',
			gateway_key_id: 'gky_1',
			model: 'gpt-4o',
			q: 'timeout',
			page: 3
		});
	});

	forEachCase(
		[
			{ name: 'drops an empty group_by rather than sending it', field: 'groupBy', value: '' },
			{ name: 'drops a whitespace-only group_by', field: 'groupBy', value: '   ' },
			{
				name: 'drops the breakdown selector\'s own word for "no breakdown"',
				field: 'groupBy',
				value: 'none'
			},
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
