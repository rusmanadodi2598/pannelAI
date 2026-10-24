// Request-log response shapes (docs/SPEC-API/001-SPEC-API.md §7.13).
//
// These are where the capture contract lives, so the cases are chosen around what would silently render
// wrong: a null `data` read as an array, a negative latency shown as a real figure, and an unknown status
// rendered verbatim. The URL half of the same screen is in `log-search`.

import { describe, expect } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	schemaConsoleLog,
	schemaLogDetail,
	schemaLogList,
	schemaLogPurge,
	schemaLogRecord
} from '$lib/schemas/log';

function record(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
		gateway_key_id: 'gky_1',
		endpoint_id: 'ep_1',
		provider_id: 'openai',
		model: 'gpt-4o',
		status: 'success',
		latency_ms: 320,
		error: 'UPSTREAM_TIMEOUT',
		has_bodies: true,
		...overrides
	};
}

const meta = { page: 1, per_page: 25, total: 1 };

describe('schemaLogRecord', () => {
	const cases = [
		{ name: 'accepts a typical row', input: record(), ok: true },
		{
			name: 'accepts a row with no optional identities',
			input: record({
				endpoint_id: undefined,
				gateway_key_id: undefined,
				provider_id: undefined,
				error: undefined,
				has_bodies: false
			}),
			ok: true
		},
		{ name: 'rejects an unknown status', input: record({ status: 'mystery' }), ok: false },
		{ name: 'rejects a negative latency', input: record({ latency_ms: -1 }), ok: false },
		{ name: 'rejects a missing has_bodies', input: record({ has_bodies: undefined }), ok: false },
		{ name: 'tolerates an additive field', input: record({ extra: 'x' }), ok: true }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaLogRecord.safeParse(testCase.input).success, testCase.name).toBe(testCase.ok);
	});
});

describe('schemaLogList', () => {
	const cases = [
		{ name: 'accepts a one-row page', input: { data: [record()], meta }, ok: true },
		{ name: 'normalizes a null data list', input: { data: null, meta }, ok: true },
		{ name: 'rejects a missing page meta', input: { data: [] }, ok: false }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaLogList.safeParse(testCase.input).success, testCase.name).toBe(testCase.ok);
	});
});

describe('schemaLogDetail', () => {
	const cases = [
		{
			name: 'accepts a row with captured bodies',
			input: {
				request_id: 'req_1',
				ts: '2026-09-17T14:03:00Z',
				status: 'success',
				latency_ms: 320,
				capture_enabled: true,
				capture_body_max_bytes: 65536,
				request_body: '{"model":"gpt-4o"}',
				response_body: '{"choices":[]}'
			},
			ok: true
		},
		{
			name: 'accepts capture off with no bodies',
			input: {
				request_id: 'req_1',
				ts: '2026-09-17T14:03:00Z',
				status: 'error',
				latency_ms: 8,
				capture_enabled: false,
				capture_body_max_bytes: 65536
			},
			ok: true
		},
		{
			name: 'rejects a missing capture_enabled',
			input: {
				request_id: 'req_1',
				ts: '2026-09-17T14:03:00Z',
				status: 'success',
				latency_ms: 320,
				capture_body_max_bytes: 65536
			},
			ok: false
		},
		{
			name: 'tolerates an additive field',
			input: {
				request_id: 'req_1',
				ts: '2026-09-17T14:03:00Z',
				status: 'success',
				latency_ms: 320,
				capture_enabled: true,
				capture_body_max_bytes: 65536,
				extra: 1
			},
			ok: true
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaLogDetail.safeParse(testCase.input).success, testCase.name).toBe(testCase.ok);
	});
});

describe('schemaLogPurge', () => {
	const cases = [
		{ name: 'accepts a positive count', input: { deleted: 42 }, ok: true },
		{ name: 'accepts zero', input: { deleted: 0 }, ok: true },
		{ name: 'rejects a negative count', input: { deleted: -1 }, ok: false }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaLogPurge.safeParse(testCase.input).success, testCase.name).toBe(testCase.ok);
	});
});

describe('schemaConsoleLog', () => {
	const cases = [
		{
			name: 'accepts lines and the bound',
			input: { lines: ['boot ok', 'request 1'], max_records: 1000 },
			ok: true
		},
		{ name: 'normalizes a null lines list', input: { lines: null, max_records: 1000 }, ok: true },
		{ name: 'rejects a negative bound', input: { lines: [], max_records: -1 }, ok: false }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaConsoleLog.safeParse(testCase.input).success, testCase.name).toBe(testCase.ok);
	});
});
