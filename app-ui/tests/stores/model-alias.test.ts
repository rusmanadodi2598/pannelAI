// Tests for the alias store (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The set is global and the write replaces it whole, so the tests are about the set a write sends rather
// than the row it changed: a store that sent one screen's slice would pass a display test and delete every
// alias the slice did not carry. Two more rules are held here because they are properties of the resource:
// a write before the first successful read would send an empty set, and the answer to a write is what the
// store must hold afterwards, not the merge it computed.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createModelAliasStore } from '$lib/stores/model-alias.svelte';
import { aliasRow, catalogRow, comboRow, stubModels, type ModelStub } from '../support/model-stub';

// The catalog the stub resolves alias targets against: the server refuses a target that is neither a known
// model nor a combo name, so a test target has to exist here for the write to land.
function stubCatalog(): Record<string, unknown>[] {
	return [
		catalogRow(),
		catalogRow({ id: 'openai/gpt-4o-mini', provider_id: 'openai', model_id: 'gpt-4o-mini' }),
		catalogRow({ id: 'anthropic/claude', provider_id: 'anthropic', model_id: 'claude' })
	];
}

let stub: ModelStub;

beforeEach(() => {
	stub = stubModels({
		catalog: stubCatalog(),
		combos: [comboRow()],
		aliases: [aliasRow({ alias: 'fast', target: 'openai/gpt-4o' })]
	});
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('load', () => {
	it('reads the whole set and marks itself ready', async () => {
		const store = createModelAliasStore();
		await store.load();

		expect(store.entries).toEqual([{ alias: 'fast', target: 'openai/gpt-4o' }]);
		expect(store.ready).toBe(true);
		expect(store.loading).toBe(false);
		expect(store.error).toBeNull();
	});

	it('reports a failed read and stays unready, which is what refuses a write', async () => {
		stub.aliasReadStatus = 500;
		const store = createModelAliasStore();
		await store.load();

		expect(store.ready).toBe(false);
		expect(store.error).toBe('The set could not be read.');
		expect(store.entries).toEqual([]);
	});

	it('clears a previous error when a retry succeeds', async () => {
		stub.aliasReadStatus = 500;
		const store = createModelAliasStore();
		await store.load();

		stub.aliasReadStatus = 200;
		await store.load();

		expect(store.error).toBeNull();
		expect(store.ready).toBe(true);
	});
});

describe('add', () => {
	it('refuses to write before the set has been read, so it cannot erase aliases it never saw', async () => {
		const store = createModelAliasStore();
		const written = await store.add({ alias: 'fast', target: 'openai/gpt-4o' });

		expect(written).toBe(false);
		expect(stub.aliasWrites).toEqual([]);
		expect(store.outcome?.ok).toBe(false);
		expect(store.outcome?.message).toContain('has not loaded');
	});

	it('adds one alias by sending the whole set, the others included', async () => {
		const store = createModelAliasStore();
		await store.load();

		const written = await store.add({ alias: 'smart', target: 'anthropic/claude' });

		expect(written).toBe(true);
		expect(stub.aliasWrites).toEqual([
			[
				{ alias: 'fast', target: 'openai/gpt-4o' },
				{ alias: 'smart', target: 'anthropic/claude' }
			]
		]);
		expect(store.entries).toEqual([
			{ alias: 'fast', target: 'openai/gpt-4o' },
			{ alias: 'smart', target: 'anthropic/claude' }
		]);
		expect(store.outcome).toEqual({ name: 'smart', ok: true, message: 'smart was added.' });
	});

	it('changes what an existing alias targets, and says so rather than claiming an add', async () => {
		const store = createModelAliasStore();
		await store.load();

		const written = await store.add({ alias: ' fast ', target: 'openai/gpt-4o-mini' });

		expect(written).toBe(true);
		// One entry, not two: the alias is the primary key, so the merge replaces the row.
		expect(stub.aliasWrites).toEqual([[{ alias: 'fast', target: 'openai/gpt-4o-mini' }]]);
		expect(store.outcome).toEqual({
			name: 'fast',
			ok: true,
			message: 'fast now targets openai/gpt-4o-mini.'
		});
	});

	it('sends the set sorted by alias, which is the order the read route answers in', async () => {
		stub.aliases = [aliasRow({ alias: 'zeta', target: 'openai/gpt-4o' })];
		const store = createModelAliasStore();
		await store.load();

		await store.add({ alias: 'alpha', target: 'fallback-combo' });

		expect(stub.aliasWrites).toEqual([
			[
				{ alias: 'alpha', target: 'fallback-combo' },
				{ alias: 'zeta', target: 'openai/gpt-4o' }
			]
		]);
	});

	it('takes the server answer as the new state rather than its own merge', async () => {
		const store = createModelAliasStore();
		await store.load();

		// The server answers with a set the panel did not send, which is what a concurrent operator's write
		// looks like from here.
		const original = globalThis.fetch;
		vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
			if ((init?.method ?? 'GET') === 'PUT') {
				return new Response(
					JSON.stringify({
						data: [
							{ alias: 'ghost', target: 'anthropic/claude' },
							{ alias: 'fast', target: 'openai/gpt-4o' }
						]
					}),
					{ status: 200, headers: { 'content-type': 'application/json' } }
				);
			}
			return (original as typeof fetch)(input as RequestInfo, init);
		});

		await store.add({ alias: 'smart', target: 'anthropic/claude' });

		// In the answer's own order, not a list the store re-sorted.
		expect(store.entries).toEqual([
			{ alias: 'ghost', target: 'anthropic/claude' },
			{ alias: 'fast', target: 'openai/gpt-4o' }
		]);
	});

	it('keeps the previous set and reports the refusal when the server answers an error', async () => {
		const store = createModelAliasStore();
		await store.load();

		stub.writeStatus = 500;
		const written = await store.add({ alias: 'smart', target: 'anthropic/claude' });

		expect(written).toBe(false);
		expect(store.entries).toEqual([{ alias: 'fast', target: 'openai/gpt-4o' }]);
		expect(store.outcome?.ok).toBe(false);
		expect(store.outcome?.message).toBe('The set could not be stored.');
	});

	it('reports the API sentence when the target resolves to nothing', async () => {
		const store = createModelAliasStore();
		await store.load();

		const written = await store.add({ alias: 'ghost', target: 'openai/ghost' });

		expect(written).toBe(false);
		expect(store.outcome?.message).toBe(
			'alias ghost targets an unknown model or combo: openai/ghost'
		);
	});

	it('names the alias a write is in flight for, so the screen can hold its controls', async () => {
		const store = createModelAliasStore();
		await store.load();

		const pending = store.add({ alias: 'smart', target: 'anthropic/claude' });
		expect(store.saving).toBe('smart');

		await pending;
		expect(store.saving).toBeNull();
	});
});

describe('remove', () => {
	it('removes one alias by sending the rest, and says what stopped resolving', async () => {
		stub.aliases = [
			aliasRow({ alias: 'fast', target: 'openai/gpt-4o' }),
			aliasRow({ alias: 'smart', target: 'anthropic/claude' })
		];
		const store = createModelAliasStore();
		await store.load();

		const written = await store.remove('smart');

		expect(written).toBe(true);
		expect(stub.aliasWrites).toEqual([[{ alias: 'fast', target: 'openai/gpt-4o' }]]);
		expect(store.entries).toEqual([{ alias: 'fast', target: 'openai/gpt-4o' }]);
		expect(store.outcome).toEqual({
			name: 'smart',
			ok: true,
			message: 'smart no longer resolves.'
		});
	});

	it('sends an empty set when the last alias goes, which is the only way to clear the set', async () => {
		const store = createModelAliasStore();
		await store.load();

		await store.remove('fast');

		expect(stub.aliasWrites).toEqual([[]]);
		expect(store.entries).toEqual([]);
	});

	it('refuses before the set has been read, so a removal cannot clear every alias', async () => {
		const store = createModelAliasStore();
		const written = await store.remove('fast');

		expect(written).toBe(false);
		expect(stub.aliasWrites).toEqual([]);
		expect(store.outcome?.ok).toBe(false);
	});
});
