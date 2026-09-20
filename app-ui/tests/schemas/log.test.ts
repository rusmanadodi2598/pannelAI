// Request-log schema and filter-parser tests (docs/SPEC-UI/001-SPEC-UI.md §6.11, §7.6;
// docs/SPEC-API/001-SPEC-API.md §7.13).
//
// The response shapes are where the capture contract lives, so the cases are chosen around what would
// silently render wrong: a null `data` read as an array, a negative latency shown as a real figure, an
// unknown status rendered verbatim, and a hand-edited URL that must correct itself rather than fail
// (SPEC-UI §7.1.1).

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	schemaConsoleLog,
	schemaLogDetail,
	schemaLogList,
	schemaLogPurge,
	schemaLogRecord
} from '$lib/schemas/log';
import { logFiltersApplied, nextLogSearch, parseLogSearch } from '$lib/schemas/log-search';

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
