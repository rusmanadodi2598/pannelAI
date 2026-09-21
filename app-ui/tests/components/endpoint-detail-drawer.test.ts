// Endpoint detail drawer render tests (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
//
// The drawer is where the endpoint's keys are actually managed, and it had no render test at all: the
// schema tests covered the shapes and nothing covered the screen. The cases below are the drawer's own
// rules, not the schema's: which key it says the router would spend and what it says when none can be,
// why the delete control is disabled on the last active key, and what happens when the gateway refuses a
// delete anyway because the state changed under the panel.
//
// The stub is local rather than the shared list stub, because the drawer reads one endpoint and writes its
// keys, and the shared stub serves the list and bulk routes. The row builders are shared, so the wire
// shapes have one definition.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import EndpointDetailDrawer from '../../src/lib/components/EndpointDetailDrawer.svelte';
import { endpointDetailRow, endpointKeyRow, endpointRow } from '../support/endpoint-stub';
import type { Endpoint } from '$lib/schemas/endpoint';

/** The list row the drawer is handed, built by the shared stub so the wire shape has one definition. */
function entryRow(overrides: Partial<Endpoint> = {}): Endpoint {
	return { ...(endpointRow() as unknown as Endpoint), ...overrides };
}

type StubOptions = {
	detail?: Record<string, unknown>;
	test?: { status: number; body?: unknown };
	deleteKey?: { status: number; body?: unknown };
	detailStatus?: number;
};

/** Serves the four calls the drawer makes and records the writes it saw. */
function stubDrawer(options: StubOptions = {}): { writes: { method: string; url: string }[] } {
	const writes: { method: string; url: string }[] = [];
	const detail = options.detail ?? endpointDetailRow();

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);
		const method = init?.method ?? 'GET';
		const json = (payload: unknown, status = 200): Response =>
			new Response(JSON.stringify(payload), {
				status,
				headers: { 'content-type': 'application/json' }
			});

		if (method !== 'GET') writes.push({ method, url });

		if (method === 'POST' && url.endsWith('/test')) {
			const answer = options.test ?? { status: 200, body: { state: 'pass', latency_ms: 88 } };
			return json(answer.body ?? {}, answer.status);
		}

		if (method === 'DELETE') {
			const answer = options.deleteKey ?? { status: 204 };
			if (answer.status === 204) return new Response(null, { status: 204 });
			return json(answer.body ?? {}, answer.status);
		}

		if (options.detailStatus && options.detailStatus !== 200) {
			return json({ error: { code: 'INTERNAL_ERROR', message: 'the detail is unavailable' } }, 500);
		}

		return json(detail);
	});

	return { writes };
}

async function openDrawer(options: StubOptions = {}): Promise<{
	writes: { method: string; url: string }[];
}> {
	const stub = stubDrawer(options);
	render(EndpointDetailDrawer, {
		props: { entry: entryRow(), onclose: vi.fn(), onchanged: vi.fn() }
	});
	await screen.findByRole('dialog');
	return stub;
}

/** The key row that carries the given hint, once the drawer has finished reading the endpoint. */
async function keyRowFor(hint: string): Promise<HTMLElement> {
	return (await screen.findByText(hint)).closest('tr') as HTMLElement;
}

/** The drawer's routing block, once the detail has loaded. */
async function routingBlock(): Promise<HTMLElement> {
	return (await screen.findByText('Which key would route now')).parentElement as HTMLElement;
}

describe('endpoint detail drawer', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('names the key the router would spend, from the fields the API returned', async () => {
		await openDrawer({
			detail: endpointDetailRow({
				keys: [
					endpointKeyRow({ id: 'uky_2', key_hint: 'sk-…2222', priority: 5 }),
					endpointKeyRow({ id: 'uky_1', key_hint: 'sk-…1111', priority: 1 })
				]
			})
		});

		const block = await routingBlock();
		// The lower priority number wins, which is the order the router spends them in.
		expect(block.textContent).toContain('sk-…1111');
		expect(block.textContent).not.toContain('sk-…2222');
	});

	it('says no key can be spent when every key is unusable, rather than naming one', async () => {
		await openDrawer({
			detail: endpointDetailRow({
				keys: [
					endpointKeyRow({ id: 'uky_1', key_hint: 'sk-…1111', status: 'disabled' }),
					endpointKeyRow({ id: 'uky_2', key_hint: 'sk-…2222', available: false })
				]
			})
		});

		const block = await routingBlock();
		expect(block.textContent).toContain('No key can be spent');
	});

	it('disables deleting the last active api_key key, and says why beside it', async () => {
		await openDrawer({
			detail: endpointDetailRow({
				auth_type: 'api_key',
				keys: [endpointKeyRow({ id: 'uky_1', key_hint: 'sk-…1111', status: 'active' })]
			})
		});

		const row = await keyRowFor('sk-…1111');
		expect(
			(within(row).getByRole('button', { name: 'Delete' }) as HTMLButtonElement).disabled
		).toBe(true);
		expect(row.textContent).toContain('Last active key');
	});

	it('still reports a CONFLICT the panel did not predict, because the state can change under it', async () => {
		const stub = await openDrawer({
			detail: endpointDetailRow({
				auth_type: 'api_key',
				keys: [
					endpointKeyRow({ id: 'uky_1', key_hint: 'sk-…1111' }),
					endpointKeyRow({ id: 'uky_2', key_hint: 'sk-…2222' })
				]
			}),
			deleteKey: {
				status: 409,
				body: {
					error: {
						code: 'CONFLICT',
						message: 'This is the last active key for the endpoint.'
					}
				}
			}
		});

		await fireEvent.click(
			within(await keyRowFor('sk-…2222')).getByRole('button', { name: 'Delete' })
		);

		expect(await screen.findByText('This is the last active key for the endpoint.')).toBeTruthy();
		expect(stub.writes[0].method).toBe('DELETE');
		expect(stub.writes[0].url).toBe('/api/v1/endpoints/ep_1/keys/uky_2');
	});

	it('tests one key through the test route and renders the answer', async () => {
		const stub = await openDrawer({
			detail: endpointDetailRow({ keys: [endpointKeyRow({ id: 'uky_1', key_hint: 'sk-…1111' })] }),
			test: { status: 200, body: { state: 'fail', latency_ms: 421, message: 'upstream said 401' } }
		});

		await fireEvent.click(
			within(await keyRowFor('sk-…1111')).getByRole('button', { name: 'Test' })
		);

		await waitFor(() => expect(stub.writes).toHaveLength(1));
		expect(stub.writes[0].url).toBe('/api/v1/endpoints/ep_1/test');
		expect(await screen.findByText(/upstream said 401/)).toBeTruthy();
	});

	it('offers both §6.2 add modes, and the repeatable row mode is one of them', async () => {
		await openDrawer();

		expect(await screen.findByRole('tab', { name: 'One key' })).toBeTruthy();
		await fireEvent.click(screen.getByRole('tab', { name: 'Several keys' }));

		expect(screen.getByText('Add several keys')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Add another row' })).toBeTruthy();
	});

	it('reports a detail it could not read, and keeps the dialog open on that state', async () => {
		await openDrawer({ detailStatus: 500 });

		expect(await screen.findByText('The endpoint could not be loaded')).toBeTruthy();
		expect(screen.getByText('the detail is unavailable')).toBeTruthy();
		expect(screen.getByRole('dialog')).toBeTruthy();
	});
});
