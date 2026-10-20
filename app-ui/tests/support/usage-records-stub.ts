// Shared fake for the Usage records tab (docs/SPEC-UI/001-SPEC-UI.md §6.5 tab 2, §8.3, §8.4.1, §8.4.2).
//
// The tab's cases are split across three files by subject (the URL and its filters, the page size and a
// failed read, the detail drawer), and all three drive the same two routes: the record list and one
// request's detail with its captured bodies. One stub, so a change to the wire shape fails all three at
// once instead of leaving one file describing a response the panel no longer gets.

import { render, screen } from '@testing-library/svelte';
import { vi } from 'vitest';
import UsageRecordsTab from '../../src/lib/components/UsageRecordsTab.svelte';
import { keyRow } from './gateway-key-stub';
import { provider } from './providers-route-stub';
import { queryOf } from './page.svelte';

export function record(overrides: Record<string, unknown> = {}): Record<string, unknown> {
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

export function logBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
		status: 'success',
		latency_ms: 320,
		capture_enabled: true,
		capture_body_max_bytes: 65536,
		request_body: '{"model":"gpt-4o"}',
		response_body: '{"choices":[]}',
		...overrides
	};
}

export type Stub = {
	requested: string[];
	records: Record<string, unknown>[];
	total: number;
	detail: Record<string, unknown>;
	status: number;
	/** The rows the filter bar's own two vocabularies answer with. */
	providers: Record<string, unknown>[];
	keys: Record<string, unknown>[];
	/** The vocabularies' refusal, which is separate from the record read's so one can fail alone. */
	optionsStatus: number;
};

export function stubRecords(overrides: Partial<Stub> = {}): Stub {
	const stub: Stub = {
		requested: [],
		records: [record()],
		total: 1,
		detail: { usage: record(), capture_enabled: true, log: logBody() },
		status: 200,
		providers: [provider()],
		keys: [keyRow()],
		optionsStatus: 200,
		...overrides
	};

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		stub.requested.push(url);

		const path = url.split('?')[0];

		// The filter bar's vocabularies are answered before the record routes' fallback, so a record read
		// that failed cannot decide what the selects offer.
		if (path.endsWith('/providers') || path.endsWith('/gateway-keys')) {
			if (stub.optionsStatus !== 200) {
				return new Response(
					JSON.stringify({
						error: { code: 'INTERNAL_ERROR', message: 'the store is unreachable' }
					}),
					{ status: stub.optionsStatus, headers: { 'content-type': 'application/json' } }
				);
			}

			const rows = path.endsWith('/providers') ? stub.providers : stub.keys;
			return new Response(
				JSON.stringify({ data: rows, meta: { page: 1, per_page: 100, total: rows.length } }),
				{ status: 200, headers: { 'content-type': 'application/json' } }
			);
		}

		const body = path.endsWith('/usage/records')
			? { data: stub.records, meta: { page: 1, per_page: 25, total: stub.total } }
			: stub.detail;

		return new Response(JSON.stringify(body), {
			status: stub.status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

export function listQuery(stub: Stub): URLSearchParams {
	const matches = stub.requested.filter((url) => url.split('?')[0].endsWith('/usage/records'));
	return new URLSearchParams(queryOf(matches[matches.length - 1] ?? ''));
}

export async function renderRecords(): Promise<void> {
	render(UsageRecordsTab);
	await screen.findByRole('table');
}
