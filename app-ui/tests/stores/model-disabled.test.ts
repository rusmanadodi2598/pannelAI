// Tests for the disabled model store (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The store is where the two halves of the screen meet, so the tests are about the set it sends rather
// than the one it shows: every write must carry the whole last-read set, because the API replaces the
// whole set. A store that sent the provider's slice would pass a display test and erase another
// provider's rows in production, which is the failure these tests exist to catch.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createModelDisabledStore } from '$lib/stores/model-disabled.svelte';
import { disabledRefKey } from '$lib/schemas/model-disabled';
import { catalogRow, stubModels, type ModelStub } from '../support/model-stub';

const ref = (provider_id: string, model_id: string): { provider_id: string; model_id: string } => ({
	provider_id,
	model_id
});

let stub: ModelStub;

beforeEach(() => {
	stub = stubModels({
		catalog: [
			catalogRow(),
			catalogRow({
				id: 'anthropic/claude-3-haiku',
				provider_id: 'anthropic',
				model_id: 'claude-3-haiku'
			})
		],
		disabled: [ref('anthropic', 'claude-3-haiku')]
	});
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('load', () => {
	it('reads the whole set and marks itself ready', async () => {
		const store = createModelDisabledStore();
		await store.load();

		expect(store.refs).toEqual([ref('anthropic', 'claude-3-haiku')]);
		expect(store.ready).toBe(true);
		expect(store.loading).toBe(false);
		expect(store.error).toBeNull();
	});

	it('reports a failed read and stays unready, which is what refuses a write', async () => {
		stub.disabledReadStatus = 500;
		const store = createModelDisabledStore();
		await store.load();

		expect(store.ready).toBe(false);
		expect(store.error).toBe('The set could not be read.');
		expect(store.refs).toEqual([]);
	});

	it('clears a previous error when a retry succeeds', async () => {
		stub.disabledReadStatus = 500;
		const store = createModelDisabledStore();
		await store.load();

		stub.disabledReadStatus = 200;
		await store.load();

		expect(store.error).toBeNull();
		expect(store.ready).toBe(true);
	});
});

describe('setDisabled', () => {
	it('refuses to write before the set has been read, so it cannot erase rows it never saw', async () => {
		const store = createModelDisabledStore();
		const written = await store.setDisabled(ref('openai', 'gpt-4o'), true);

		expect(written).toBe(false);
		expect(stub.disabledWrites).toEqual([]);
		expect(store.outcome?.ok).toBe(false);
		expect(store.outcome?.message).toContain('has not loaded');
	});

	it('disables one model by sending the whole set, another provider included', async () => {
		const store = createModelDisabledStore();
		await store.load();

		const written = await store.setDisabled(ref('openai', 'gpt-4o'), true);

		expect(written).toBe(true);
		expect(stub.disabledWrites).toEqual([
			[ref('anthropic', 'claude-3-haiku'), ref('openai', 'gpt-4o')]
		]);
		expect(store.refs).toEqual([ref('anthropic', 'claude-3-haiku'), ref('openai', 'gpt-4o')]);
		expect(store.outcome).toEqual({
			key: 'openai/gpt-4o',
			ok: true,
			message: 'openai/gpt-4o is disabled.'
		});
	});

	it('enables one model by sending the whole set without it', async () => {
		const store = createModelDisabledStore();
		await store.load();

		const written = await store.setDisabled(ref('anthropic', 'claude-3-haiku'), false);

		expect(written).toBe(true);
		expect(stub.disabledWrites).toEqual([[]]);
		expect(store.refs).toEqual([]);
		expect(store.outcome).toEqual({
			key: 'anthropic/claude-3-haiku',
			ok: true,
			message: 'anthropic/claude-3-haiku is routable again.'
		});
	});

	it('takes the server answer as the new state rather than its own merge', async () => {
		const store = createModelDisabledStore();
		await store.load();

		// The server answers with a pair the panel did not send, which is what a concurrent operator's
		// write looks like from here.
		const original = globalThis.fetch;
		vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
			if ((init?.method ?? 'GET') === 'PUT') {
				return new Response(
					JSON.stringify({ data: [ref('openai', 'gpt-4o'), ref('google', 'gemini-2.0')] }),
					{ status: 200, headers: { 'content-type': 'application/json' } }
				);
			}
			return (original as typeof fetch)(input as RequestInfo, init);
		});

		await store.setDisabled(ref('openai', 'gpt-4o'), true);

		// In the answer's own order, which in production is the repository's `ORDER BY provider_id,
		// model_id`: the panel renders the server's list, not a list it re-sorted.
		expect(store.refs).toEqual([ref('openai', 'gpt-4o'), ref('google', 'gemini-2.0')]);
	});

	it('keeps the previous set and reports the refusal when the server answers an error', async () => {
		const store = createModelDisabledStore();
		await store.load();

		stub.writeStatus = 500;
		const written = await store.setDisabled(ref('openai', 'gpt-4o'), true);

		expect(written).toBe(false);
		expect(store.refs).toEqual([ref('anthropic', 'claude-3-haiku')]);
		expect(store.outcome?.ok).toBe(false);
		expect(store.outcome?.message).toBe('The set could not be stored.');
	});

	it('names the ref a write is in flight for, so the row can say so', async () => {
		const store = createModelDisabledStore();
		await store.load();

		const pending = store.setDisabled(ref('openai', 'gpt-4o'), true);
		expect(store.saving).toBe('openai/gpt-4o');

		await pending;
		expect(store.saving).toBeNull();
	});
});

describe('mine', () => {
	it('narrows to one provider without changing what the store holds', async () => {
		const store = createModelDisabledStore();
		await store.load();

		expect(store.mine('anthropic')).toEqual([ref('anthropic', 'claude-3-haiku')]);
		expect(store.mine('openai')).toEqual([]);
		expect(store.refs).toHaveLength(1);
	});

	it('keys its rows the same way the set does', () => {
		expect(disabledRefKey(ref('openai', 'gpt-4o'))).toBe('openai/gpt-4o');
	});
});
