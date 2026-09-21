// Shared fake API for the upstream endpoint screens (docs/SPEC-API/001-SPEC-API.md §7.5).
//
// The stub applies the list filters to its own rows and pages the result, because the rule this slice turns
// on is a rule about where filtering happens: a panel that narrowed one page in the browser would pass
// against a stub that returned everything, and fail against this one.
//
// It answers both reads the upstream tab makes. The page read carries the filters and the screen's page
// size; the option read asks for the API's per_page cap and carries no filter, so the stub tells them apart
// by that parameter rather than by call order, and a test can make one fail without the other.
//
// The bulk route answers both shapes the API distinguishes: a created set, and an all-or-nothing refusal
// that still reports every row by index with the message on the offending one (§8.1).

import { vi } from 'vitest';

/** The instant every key this stub mints is created at, so a test can tell it from the read time. */
export const KEY_CREATED_AT = '2026-09-21T09:00:00Z';

export function endpointRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'ep_1',
		provider_id: 'openai',
		provider_name: 'OpenAI',
		label: 'Primary',
		auth_type: 'api_key',
		priority: 1,
		status: 'active',
		account: {},
		key_count: 2,
		active_key_count: 2,
		available: true,
		created_at: '2026-09-17T10:00:00Z',
		updated_at: '2026-09-17T10:00:00Z',
		...overrides
	};
}

export function endpointKeyRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'uky_1',
		endpoint_id: 'ep_1',
		label: 'First',
		key_hint: 'sk-...a1',
		priority: 1,
		status: 'active',
		available: true,
		consecutive_errors: 0,
		created_at: '2026-09-17T10:00:00Z',
		updated_at: '2026-09-17T10:00:00Z',
		...overrides
	};
}

/** A detail row, which is the list row plus the keys the drawer reads. */
export function endpointDetailRow(
	overrides: Record<string, unknown> = {}
): Record<string, unknown> {
	return { ...endpointRow(), keys: [endpointKeyRow()], ...overrides };
}

/** What the bulk route answers: a created set, or a refusal that names one row. */
export type BulkAnswer =
	| { kind: 'created'; count: number }
	| { kind: 'refused'; index: number; code: string; message: string };

export type EndpointStub = {
	requested: string[];
	rows: Record<string, unknown>[];
	detail: Record<string, unknown>;
	listStatus: number;
	/** The unfiltered option read's own status, for the test that needs it to fail on its own. */
	optionsStatus: number;
	detailStatus: number;
	testAnswer: Record<string, unknown>;
	keyWrites: { endpointId: string; body: Record<string, unknown> }[];
	bulkWrites: { endpointId: string; body: Record<string, unknown> }[];
	bulkAnswer: BulkAnswer;
};

export function stubEndpoints(overrides: Partial<EndpointStub> = {}): EndpointStub {
	const stub: EndpointStub = {
		requested: [],
		rows: [endpointRow()],
		detail: endpointDetailRow(),
		listStatus: 200,
		optionsStatus: 200,
		detailStatus: 200,
		testAnswer: { state: 'pass', latency_ms: 120 },
		keyWrites: [],
		bulkWrites: [],
		bulkAnswer: { kind: 'created', count: 1 },
		...overrides
	};

	const json = (payload: unknown, status = 200): Response =>
		new Response(JSON.stringify(payload), {
			status,
			headers: { 'content-type': 'application/json' }
		});

	// The rows a list read answers with: the stub's own filter, applied server-side, then paged.
	function listResponse(params: URLSearchParams): Record<string, unknown> {
		const matched = stub.rows.filter(
			(row) =>
				(params.get('provider_id') === null || row.provider_id === params.get('provider_id')) &&
				(params.get('status') === null || row.status === params.get('status'))
		);
		const perPage = Number(params.get('per_page') ?? '25');
		const page = Number(params.get('page') ?? '1');
		const start = (page - 1) * perPage;

		return {
			data: matched.slice(start, start + perPage),
			meta: { page, per_page: perPage, total: matched.length }
		};
	}

	function createdKeys(
		endpointId: string,
		body: Record<string, unknown>
	): Record<string, unknown>[] {
		const rows = Array.isArray(body.keys) ? (body.keys as Record<string, unknown>[]) : [];
		return rows.map((row, index) =>
			endpointKeyRow({
				id: `uky_new${index}`,
				endpoint_id: endpointId,
				label: typeof row.label === 'string' ? row.label : '',
				key_hint: `sk-...n${index}`,
				priority: typeof row.priority === 'number' ? row.priority : 1,
				created_at: KEY_CREATED_AT,
				updated_at: KEY_CREATED_AT
			})
		);
	}

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';
		const url = String(input);
		const body: Record<string, unknown> =
			init?.body === undefined ? {} : JSON.parse(String(init.body));
		const parsed = new URL(url, 'http://panel.test');
		stub.requested.push(url);

		const bulkMatch = /\/endpoints\/([^/]+)\/keys\/bulk$/.exec(parsed.pathname);
		if (method === 'POST' && bulkMatch) {
			const endpointId = decodeURIComponent(bulkMatch[1]);
			stub.bulkWrites.push({ endpointId, body });

			if (stub.bulkAnswer.kind === 'created') {
				const created = createdKeys(endpointId, body);
				return json(
					{
						created,
						results: created.map((key, index) => ({ index, id: key.id }))
					},
					201
				);
			}

			const { index, code, message } = stub.bulkAnswer;
			const rowCount = Array.isArray(body.keys) ? body.keys.length : 0;
			return json(
				{
					error: { code, message },
					results: Array.from({ length: rowCount }, (_, position) =>
						position === index ? { index: position, error: message } : { index: position }
					)
				},
				400
			);
		}

		const keyMatch = /\/endpoints\/([^/]+)\/keys$/.exec(parsed.pathname);
		if (method === 'POST' && keyMatch) {
			const endpointId = decodeURIComponent(keyMatch[1]);
			stub.keyWrites.push({ endpointId, body });
			return json(createdKeys(endpointId, { keys: [body] })[0], 201);
		}

		const testMatch = /\/endpoints\/([^/]+)\/test$/.exec(parsed.pathname);
		if (method === 'POST' && testMatch) {
			return json(stub.testAnswer);
		}

		if (parsed.pathname.endsWith('/endpoints')) {
			const isOptionRead = parsed.searchParams.get('per_page') === '100';
			if (isOptionRead && stub.optionsStatus !== 200) {
				return json(
					{ error: { code: 'INTERNAL_ERROR', message: 'list failed' } },
					stub.optionsStatus
				);
			}
			if (!isOptionRead && stub.listStatus !== 200) {
				return json({ error: { code: 'INTERNAL_ERROR', message: 'list failed' } }, stub.listStatus);
			}
			return json(listResponse(parsed.searchParams));
		}

		const detailMatch = /\/endpoints\/([^/]+)$/.exec(parsed.pathname);
		if (method === 'GET' && detailMatch) {
			if (stub.detailStatus !== 200) {
				return json(
					{ error: { code: 'INTERNAL_ERROR', message: 'detail failed' } },
					stub.detailStatus
				);
			}
			return json(stub.detail);
		}

		return json({ error: { code: 'NOT_FOUND', message: 'no stub route' } }, 404);
	});

	return stub;
}
