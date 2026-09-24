// Shared fetch stub for the gateway keys tab and its row actions.
//
// The rows are built the way `app-serv` serves them, and the differences matter: `plaintext_key` is the
// field name the create response uses, the update response carries no plaintext at all, and a key that was
// never used omits `last_used_at` rather than sending null. A stub that answered with the shapes the panel
// used to demand would keep hiding the mismatches the render tests exist to catch.

import { vi } from 'vitest';

/** A key row as the gateway serves it: no `last_used_at` at all until the key has been used once. */
export function keyRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'gky_1',
		name: 'Laptop',
		key_hint: 'sk-…abcd',
		status: 'active',
		request_count: 12,
		created_at: '2026-09-18T00:00:00Z',
		...overrides
	};
}

export type GatewayKeyStubOptions = {
	rows?: Record<string, unknown>[];
	create?: { status: number; body?: unknown };
	update?: { status: number; body?: unknown };
	revoke?: { status: number; body?: unknown };
	listStatus?: number;
};

export type GatewayKeyStub = {
	/** Every read the screen made, in order, so a refresh control can be told from a re-render. */
	reads: string[];
	/** Every write the screen made, in order, with the body it sent. */
	writes: { method: string; url: string; body: unknown }[];
};

/** Answers the four routes the tab uses and records the reads and the writes it saw. */
export function stubGatewayKeys(options: GatewayKeyStubOptions = {}): GatewayKeyStub {
	const reads: GatewayKeyStub['reads'] = [];
	const writes: GatewayKeyStub['writes'] = [];
	const rows = options.rows ?? [keyRow()];

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);
		const method = init?.method ?? 'GET';
		const json = (payload: unknown, status = 200): Response =>
			new Response(JSON.stringify(payload), {
				status,
				headers: { 'content-type': 'application/json' }
			});

		if (method === 'GET') {
			reads.push(url);
		} else {
			writes.push({ method, url, body: init?.body ? JSON.parse(String(init.body)) : undefined });
		}

		if (method === 'POST') {
			const answer = options.create ?? {
				status: 201,
				body: {
					id: 'gky_2',
					name: 'CI runner',
					plaintext_key: 'sk-live-once-9999',
					key_hint: 'sk-…9999',
					created_at: '2026-09-21T00:00:00Z'
				}
			};
			return json(answer.body ?? {}, answer.status);
		}

		if (method === 'PATCH') {
			const answer = options.update ?? { status: 200, body: keyRow({ name: 'Laptop two' }) };
			return json(answer.body ?? {}, answer.status);
		}

		if (method === 'DELETE') {
			const answer = options.revoke ?? { status: 204 };
			if (answer.status === 204) {
				// The route is a soft revocation: the key stays in the list carrying the terminal status,
				// and the screen is what hides it. Modelling that here is what lets a delete test assert
				// the row leaves the table after the next read.
				const id = url.split('/').pop();
				const row = rows.find((candidate) => candidate.id === id);
				if (row) row.status = 'revoked';
				return new Response(null, { status: 204 });
			}
			return json(answer.body ?? {}, answer.status);
		}

		if (options.listStatus && options.listStatus !== 200) {
			return json({ error: { code: 'INTERNAL_ERROR', message: 'the list is unavailable' } }, 500);
		}

		return json({ data: rows, meta: { page: 1, per_page: 25, total: rows.length } });
	});

	return { reads, writes };
}
