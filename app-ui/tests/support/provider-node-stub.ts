// Shared fixture and fake API for the custom provider node tests (docs/SPEC-API/001-SPEC-API.md §7.4,
// docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The stub stores nodes and applies every write to them, because the rule this slice turns on is a rule
// about state: the panel re-reads the set after a write (§8.6.3), and a stub that answered the same
// document no matter what was sent would let a screen that never re-reads pass.
//
// Two refusals the gateway makes are expressible rather than assumed, because both are answers the panel
// has to render rather than predict: a prefix another provider already uses (CONFLICT), and a delete
// while an endpoint still references the node (CONFLICT). The probe's answer is a state and a latency,
// never an HTTP failure, which is what the route does with a credential it refuses.

import { vi } from 'vitest';

export function nodeRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'openai-compatible-01J',
		type: 'openai-compatible',
		name: 'Corp gateway',
		prefix: 'mycorp',
		api_type: 'chat',
		base_url: 'https://llm.example.com/v1',
		format: 'openai',
		created_at: '2026-09-22T03:00:00Z',
		updated_at: '2026-09-22T03:00:00Z',
		...overrides
	};
}

export type NodeStub = {
	nodes: Record<string, unknown>[];
	/** Every read the screen made, in order, so a refresh control can be told from a re-render. */
	reads: string[];
	creates: Record<string, unknown>[];
	patches: { id: string; body: Record<string, unknown> }[];
	deletes: string[];
	tests: { id: string; body: Record<string, unknown> }[];
	readStatus: number;
	writeStatus: number;
	/** Prefixes whose create is refused, so a collision is expressible. */
	takenPrefixes: string[];
	/** Node ids whose delete is refused because an endpoint still points at them. */
	referenced: string[];
	testState: string;
	testMessage: string;
};

export function stubProviderNodes(overrides: Partial<NodeStub> = {}): NodeStub {
	const stub: NodeStub = {
		nodes: [],
		reads: [],
		creates: [],
		patches: [],
		deletes: [],
		tests: [],
		readStatus: 200,
		writeStatus: 200,
		takenPrefixes: [],
		referenced: [],
		testState: 'ok',
		testMessage: '',
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

	const find = (id: string): Record<string, unknown> | undefined =>
		stub.nodes.find((row) => row.id === id);

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';
		const url = String(input);
		const body: Record<string, unknown> =
			init?.body === undefined ? {} : JSON.parse(String(init.body));

		if (method === 'GET') stub.reads.push(url);

		// The probe route is matched before the stored one, because `/provider-nodes/x/test` also matches
		// the stored route's shape at a glance and the order is what keeps them apart.
		const probe = /\/provider-nodes\/([^/]+)\/test$/.exec(url);
		if (method === 'POST' && probe) {
			const id = decodeURIComponent(probe[1]);
			stub.tests.push({ id, body });

			const row = find(id);
			if (!row) return refusal('NOT_FOUND', 'That provider no longer exists.', 404);

			return json({
				state: stub.testState,
				latency_ms: stub.testState === 'ok' ? 42 : 5000,
				checked_at: '2026-09-22T05:00:00Z',
				...(stub.testMessage === '' ? {} : { message: stub.testMessage })
			});
		}

		if (url.endsWith('/provider-nodes')) {
			if (method === 'POST') {
				stub.creates.push(body);
				if (stub.writeStatus !== 200) {
					return refusal('VALIDATION_ERROR', 'The gateway refused this value.', stub.writeStatus);
				}
				if (stub.takenPrefixes.includes(String(body.prefix))) {
					return refusal('CONFLICT', `prefix "${body.prefix}" is already used by "openai"`, 409);
				}

				const created = nodeRow({
					id: `${body.type}-created${++minted}`,
					type: body.type,
					name: body.name,
					prefix: body.prefix,
					base_url: body.base_url,
					format: body.type === 'anthropic-compatible' ? 'claude' : 'openai'
				});
				// §7.4's Anthropic-compatible response carries no api type at all, so the fake one must not
				// either: a row that always had `chat` on it would let a panel that always renders the field
				// pass, and a real node would render an empty value there.
				if (body.type === 'anthropic-compatible') delete created.api_type;
				else if (body.api_type !== undefined) created.api_type = body.api_type;
				stub.nodes.push(created);
				return json(created, 201);
			}

			if (stub.readStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The node store is unreachable.', stub.readStatus);
			}
			return json({ data: stub.nodes });
		}

		const byId = /\/provider-nodes\/([^/]+)$/.exec(url);
		if (byId) {
			const id = decodeURIComponent(byId[1]);
			const row = find(id);

			if (method === 'DELETE') {
				stub.deletes.push(id);
				if (stub.writeStatus !== 200) {
					return refusal('INTERNAL_ERROR', 'The provider was not deleted.', stub.writeStatus);
				}
				if (stub.referenced.includes(id)) {
					return refusal('CONFLICT', 'an endpoint still references this provider', 409);
				}
				if (!row) return refusal('NOT_FOUND', 'That provider no longer exists.', 404);

				stub.nodes = stub.nodes.filter((entry) => entry.id !== id);
				return new Response(null, { status: 204 });
			}

			if (method === 'PATCH') {
				stub.patches.push({ id, body });
				if (stub.writeStatus !== 200) {
					return refusal('VALIDATION_ERROR', 'The gateway refused this value.', stub.writeStatus);
				}
				if (!row) return refusal('NOT_FOUND', 'That provider no longer exists.', 404);

				Object.assign(row, body, { updated_at: '2026-09-22T06:00:00Z' });
				return json(row);
			}

			if (!row) return refusal('NOT_FOUND', 'That provider no longer exists.', 404);
			if (stub.readStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The node store is unreachable.', stub.readStatus);
			}
			return json(row);
		}

		return refusal('NOT_FOUND', 'No route matches that request.', 404);
	});

	return stub;
}
