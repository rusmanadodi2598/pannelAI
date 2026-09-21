// Shared fixture and fake API for the Proxy Pools screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.9).
//
// The stub keeps a pool and applies every write to it, so the re-read the screen performs after a
// write answers with what the write stored. A stub that returned the same document no matter what
// was sent would let a screen that never re-reads pass, and re-reading is the rule §8.6.3 exists to
// enforce.
//
// A stored test writes its result into the row as well, because that is what the API does: the list
// and the test answer agree, and the row's "last test" column is the stored result rather than the
// last thing the page happened to see.
//
// Reading a control's state is `tests/support/dom.ts`, which the component tests share.

import { vi } from 'vitest';
import { settingsDocument } from './settings-document';

export function proxyRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'prx_01HZZ9K2',
		label: 'Frankfurt egress',
		protocol: 'https',
		host: 'proxy.example.com',
		port: 8443,
		username: 'operator',
		has_password: true,
		enabled: true,
		created_at: '2026-09-19T09:00:00Z',
		updated_at: '2026-09-19T09:05:00Z',
		...overrides
	};
}

export type ProxyStub = {
	pool: Record<string, unknown>[];
	settings: Record<string, unknown>;
	/** Every read the screen made, in order, so a refresh control can be told from a re-render. */
	reads: string[];
	creates: Record<string, unknown>[];
	patches: { id: string; body: Record<string, unknown> }[];
	deletes: string[];
	testedIds: string[];
	candidateTests: Record<string, unknown>[];
	settingsPatches: Record<string, unknown>[];
	readStatus: number;
	writeStatus: number;
	testState: string;
	testMessage: string;
	settingsStatus: number;
	/** Hosts whose create is refused, so a partial batch failure is expressible. */
	refuseHosts: string[];
};

export function stubProxies(overrides: Partial<ProxyStub> = {}): ProxyStub {
	const stub: ProxyStub = {
		pool: [],
		settings: settingsDocument(),
		reads: [],
		creates: [],
		patches: [],
		deletes: [],
		testedIds: [],
		candidateTests: [],
		settingsPatches: [],
		readStatus: 200,
		writeStatus: 200,
		testState: 'ok',
		testMessage: '',
		settingsStatus: 200,
		refuseHosts: [],
		...overrides
	};

	let minted = 0;

	const json = (payload: unknown, status = 200): Response =>
		new Response(JSON.stringify(payload), {
			status,
			headers: { 'content-type': 'application/json' }
		});
	const refusal = (code: string, message: string, status: number): Response =>
		json({ error: { code, message } }, status);

	// The probe answer. A fail carries the reason, because a bare state would send an operator looking
	// for a cause the gateway already knows.
	const testAnswer = (): Record<string, unknown> => ({
		state: stub.testState,
		latency_ms: stub.testState === 'ok' ? 42 : 5000,
		checked_at: '2026-09-19T10:00:00Z',
		...(stub.testMessage === '' ? {} : { message: stub.testMessage })
	});

	const find = (id: string): Record<string, unknown> | undefined =>
		stub.pool.find((row) => row.id === id);

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';
		const url = String(input);
		const body: Record<string, unknown> =
			init?.body === undefined ? {} : JSON.parse(String(init.body));

		if (method === 'GET') stub.reads.push(url);

		if (url.includes('/settings')) {
			if (method === 'PATCH') {
				stub.settingsPatches.push(body);
				if (stub.writeStatus !== 200) {
					return refusal('VALIDATION_ERROR', 'The gateway refused this value.', stub.writeStatus);
				}
				stub.settings = { ...stub.settings, ...body };
				return json(stub.settings);
			}
			if (stub.settingsStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The settings store is unreachable.', stub.settingsStatus);
			}
			return json(stub.settings);
		}

		// The candidate route is checked before the stored one, because `/proxies/test` also matches the
		// stored route's shape at a glance and the order is what keeps them apart.
		if (method === 'POST' && url.endsWith('/proxies/test')) {
			stub.candidateTests.push(body);
			return json(testAnswer());
		}

		const storedTest = /\/proxies\/([^/]+)\/test$/.exec(url);
		if (method === 'POST' && storedTest) {
			const id = decodeURIComponent(storedTest[1]);
			stub.testedIds.push(id);

			const row = find(id);
			if (!row) return refusal('NOT_FOUND', 'That proxy no longer exists.', 404);

			row.status = testAnswer();
			return json(testAnswer());
		}

		if (url.endsWith('/proxies')) {
			if (method === 'POST') {
				stub.creates.push(body);
				if (stub.writeStatus !== 200) {
					return refusal('VALIDATION_ERROR', 'The gateway refused this value.', stub.writeStatus);
				}
				if (stub.refuseHosts.includes(String(body.host))) {
					return refusal('VALIDATION_ERROR', 'That address is already in the pool.', 400);
				}

				const created = {
					id: `prx_created${++minted}`,
					label: body.label,
					protocol: body.protocol,
					host: body.host,
					port: body.port,
					username: body.username ?? '',
					has_password: (body.password ?? '') !== '',
					enabled: body.enabled ?? true,
					created_at: '2026-09-19T10:00:00Z',
					updated_at: '2026-09-19T10:00:00Z'
				};
				stub.pool.push(created);
				return json(created, 201);
			}

			if (stub.readStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The pool store is unreachable.', stub.readStatus);
			}
			return json({ data: stub.pool });
		}

		const byId = /\/proxies\/([^/]+)$/.exec(url);
		if (byId) {
			const id = decodeURIComponent(byId[1]);
			const row = find(id);

			if (method === 'DELETE') {
				stub.deletes.push(id);
				if (stub.writeStatus !== 200) {
					return refusal('INTERNAL_ERROR', 'The proxy was not deleted.', stub.writeStatus);
				}
				stub.pool = stub.pool.filter((entry) => entry.id !== id);
				return new Response(null, { status: 204 });
			}

			if (method === 'PATCH') {
				stub.patches.push({ id, body });
				if (stub.writeStatus !== 200) {
					return refusal('VALIDATION_ERROR', 'The gateway refused this value.', stub.writeStatus);
				}
				if (!row) return refusal('NOT_FOUND', 'That proxy no longer exists.', 404);

				// An empty password keeps the stored secret, which is the rule the panel relies on when
				// it sends the field back empty after an edit.
				const password = body.password ?? '';
				const updated: Record<string, unknown> = {
					...row,
					...body,
					has_password: password === '' ? row.has_password : true,
					updated_at: '2026-09-19T10:05:00Z'
				};
				// The real patch answer carries no password field, so the fake one must not either.
				delete updated.password;
				Object.assign(row, updated);
				return json(row);
			}
		}

		return refusal('NOT_FOUND', 'No route matches that request.', 404);
	});

	return stub;
}
