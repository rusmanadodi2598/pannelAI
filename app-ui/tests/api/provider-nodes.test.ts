// Tests for the custom provider node client (src/lib/api/provider-nodes.ts).
//
// The paths and the bodies are what this file is about, because they are the part the panel can get
// wrong silently: a PATCH that carries `api_type` is refused by the gateway, and a test that sends an
// empty credential tests the wrong thing. Every case reads the request the client actually built.

import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	createProviderNode,
	deleteProviderNode,
	getProviderNode,
	listProviderNodes,
	testProviderNode,
	updateProviderNode
} from '$lib/api/provider-nodes';

type Call = { url: string; method: string; body: unknown };

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}

const NODE = {
	id: 'openai-compatible-01J',
	type: 'openai-compatible',
	name: 'Prod node',
	prefix: 'mycorp',
	api_type: 'chat',
	base_url: 'https://llm.example.com/v1',
	format: 'openai',
	created_at: '2026-09-22T03:00:00Z',
	updated_at: '2026-09-22T03:00:00Z'
};

/** Answers every call with one body and records what was sent. */
function stub(respond: (call: Call) => Response): { calls: Call[] } {
	const calls: Call[] = [];

	vi.stubGlobal('fetch', async (input: unknown, init: RequestInit = {}) => {
		const call = {
			url: String(input),
			method: init.method ?? 'GET',
			body: init.body === undefined ? undefined : JSON.parse(String(init.body))
		};
		calls.push(call);
		return respond(call);
	});

	return { calls };
}

describe('custom provider node client', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('reads the whole node set, which is not paginated', async () => {
		const stubber = stub(() => jsonResponse(200, { data: [NODE] }));

		const result = await listProviderNodes();

		expect(stubber.calls[0]?.method).toBe('GET');
		expect(stubber.calls[0]?.url).toBe('/api/v1/provider-nodes');
		expect(stubber.calls[0]?.body).toBeUndefined();
		expect(result.ok && result.data.data[0]?.id).toBe('openai-compatible-01J');
	});

	it('creates a node with the body it was handed', async () => {
		const stubber = stub(() => jsonResponse(201, NODE));

		await createProviderNode({
			name: 'Prod node',
			prefix: 'mycorp',
			type: 'openai-compatible',
			api_type: 'chat',
			base_url: 'https://llm.example.com/v1'
		});

		expect(stubber.calls[0]?.method).toBe('POST');
		expect(stubber.calls[0]?.url).toBe('/api/v1/provider-nodes');
		expect(stubber.calls[0]?.body).toEqual({
			name: 'Prod node',
			prefix: 'mycorp',
			type: 'openai-compatible',
			api_type: 'chat',
			base_url: 'https://llm.example.com/v1'
		});
	});

	it('reports a refused create as the gateway stated it, naming the owner of a taken prefix', async () => {
		stub(() =>
			jsonResponse(409, {
				error: { code: 'CONFLICT', message: 'prefix "openai" is already used by "openai"' }
			})
		);

		const result = await createProviderNode({
			name: 'Prod node',
			prefix: 'openai',
			type: 'openai-compatible',
			api_type: 'chat',
			base_url: 'https://llm.example.com/v1'
		});

		expect(result.ok).toBe(false);
		expect(!result.ok && result.error.code).toBe('CONFLICT');
		expect(!result.ok && result.error.message).toContain('already used by');
	});

	it('reads one node by its id, escaped', async () => {
		const stubber = stub(() => jsonResponse(200, NODE));

		await getProviderNode('openai-compatible-01J');

		expect(stubber.calls[0]?.method).toBe('GET');
		expect(stubber.calls[0]?.url).toBe('/api/v1/provider-nodes/openai-compatible-01J');
	});

	it('patches only the fields it was handed, so no identity field rides along', async () => {
		const stubber = stub(() => jsonResponse(200, NODE));

		await updateProviderNode('openai-compatible-01J', {
			name: 'Renamed',
			prefix: 'mycorp',
			base_url: 'https://llm.example.com/v1'
		});

		expect(stubber.calls[0]?.method).toBe('PATCH');
		expect(stubber.calls[0]?.body).toEqual({
			name: 'Renamed',
			prefix: 'mycorp',
			base_url: 'https://llm.example.com/v1'
		});
	});

	it('deletes a node and accepts the empty 204 the route answers with', async () => {
		const stubber = stub(() => new Response(null, { status: 204 }));

		const result = await deleteProviderNode('openai-compatible-01J');

		expect(stubber.calls[0]?.method).toBe('DELETE');
		expect(result.ok).toBe(true);
	});

	it('reports the refusal that keeps a referenced node alive', async () => {
		stub(() =>
			jsonResponse(409, {
				error: { code: 'CONFLICT', message: 'an endpoint still references this provider' }
			})
		);

		const result = await deleteProviderNode('openai-compatible-01J');

		expect(!result.ok && result.error.code).toBe('CONFLICT');
	});

	it('sends no credential when none was typed, which is what tests a node needing none', async () => {
		const stubber = stub(() =>
			jsonResponse(200, { state: 'ok', latency_ms: 12, checked_at: '2026-09-22T03:00:00Z' })
		);

		const result = await testProviderNode('openai-compatible-01J');

		expect(stubber.calls[0]?.method).toBe('POST');
		expect(stubber.calls[0]?.url).toBe('/api/v1/provider-nodes/openai-compatible-01J/test');
		expect(stubber.calls[0]?.body).toEqual({});
		expect(result.ok && result.data.state).toBe('ok');
	});

	it('sends a whitespace credential, which is a value the operator typed', async () => {
		const stubber = stub(() => jsonResponse(200, { state: 'ok', latency_ms: 3 }));

		await testProviderNode('openai-compatible-01J', '   ');

		// Only the empty field means "no credential"; the gateway decides what a blank one is worth.
		expect(stubber.calls[0]?.body).toEqual({ credential: '   ' });
	});

	it('sends no credential for the empty field either', async () => {
		const stubber = stub(() => jsonResponse(200, { state: 'ok', latency_ms: 3 }));

		await testProviderNode('openai-compatible-01J', '');

		expect(stubber.calls[0]?.body).toEqual({});
	});

	it('sends a credential the operator typed', async () => {
		const stubber = stub(() =>
			jsonResponse(200, { state: 'fail', latency_ms: 40, message: '401' })
		);

		const result = await testProviderNode('openai-compatible-01J', 'sk-test');

		expect(stubber.calls[0]?.body).toEqual({ credential: 'sk-test' });
		expect(result.ok && result.data.state).toBe('fail');
		expect(result.ok && result.data.message).toBe('401');
	});
});
