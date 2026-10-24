// The Logs screen's fetch harness, shared by the tests that render its requests tab.
//
// The stub records every URL it was asked for, because the rule under test is usually which query the
// panel built from the URL rather than what a selector's label says.

import { vi } from 'vitest';
import { queryOf } from './page.svelte';

export function record(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
		gateway_key_id: 'gky_1',
		endpoint_id: 'ep_1',
		provider_id: 'openai',
		model: 'gpt-4o',
		status: 'success',
		latency_ms: 320,
		error: '',
		has_bodies: true,
		...overrides
	};
}

export function detail(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
		gateway_key_id: 'gky_1',
		endpoint_id: 'ep_1',
		provider_id: 'openai',
		model: 'gpt-4o',
		status: 'success',
		latency_ms: 320,
		capture_enabled: true,
		capture_body_max_bytes: 65536,
		request_body: '{"model":"gpt-4o"}',
		response_body: '{"choices":[]}',
		...overrides
	};
}

type Stub = {
	requested: string[];
	records: Record<string, unknown>[];
	total: number;
	detail: Record<string, unknown>;
	status: number;
};

export function stubLogs(overrides: Partial<Stub> = {}): Stub {
	const stub: Stub = {
		requested: [],
		records: [record()],
		total: 1,
		detail: detail(),
		status: 200,
		...overrides
	};

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		stub.requested.push(url);

		const path = url.split('?')[0];
		const body =
			path.includes('/logs/requests/') && !path.endsWith('requests')
				? stub.detail
				: { data: stub.records, meta: { page: 1, per_page: 25, total: stub.total } };

		return new Response(JSON.stringify(body), {
			status: stub.status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

/** The query of the last list read the tab asked for. */
export function listQuery(stub: Stub): URLSearchParams {
	const matches = stub.requested.filter((url) => url.split('?')[0].endsWith('/logs/requests'));
	return new URLSearchParams(queryOf(matches[matches.length - 1] ?? ''));
}
