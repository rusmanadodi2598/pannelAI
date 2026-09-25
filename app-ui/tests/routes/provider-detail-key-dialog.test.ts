// The key dialog on the provider detail page (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
//
// What this screen owns is the concept, so that is what these cases hold: a key added here becomes a
// connection of this provider, one row per key, and the operator is never asked to create a container
// first. A single key is one `POST /endpoints`; a paste is one `POST /endpoints/bulk` carrying one element
// per line, which is the reference's own shape (`providers/[id]/AddApiKeyModal.js:148-182`, one connection
// per key).
//
// The dialog's modes are driven through the page rather than mounted on their own, because what has to
// hold is the path from this screen to the wire and back into the table below it.
//
// The stub refuses the two things the API refuses, a row with no key and a label already stored, so a
// case that presses the button with an empty field proves the panel caught it rather than that the stub
// was lenient.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { endpointRow } from '../support/endpoint-stub';
import { stubModels } from '../support/model-stub';
import { value } from '../support/dom';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function renderProvider(): void {
	render(ProviderDetailPage, { props: { params: { provider_id: 'openai' }, data: {} } });
}

/** Types into a field by its label, through the event Svelte 5's `bind:value` listens for. */
async function fill(label: string, text: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText(label), { target: { value: text } });
}

async function openDialog(): Promise<void> {
	await fireEvent.click(await screen.findByRole('button', { name: 'Add API Key' }));
	await screen.findByLabelText('Name');
}

async function openPaste(): Promise<void> {
	await openDialog();
	await fireEvent.click(screen.getByRole('tab', { name: 'Bulk Add' }));
	await screen.findByLabelText('Keys');
}

describe('the key control on a provider', () => {
	it('is offered where the provider takes a key, under the connections heading', async () => {
		stubModels({ authType: 'api_key' });
		renderProvider();

		expect(await screen.findByRole('button', { name: 'Add API Key' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'Connections' })).toBeTruthy();
		expect(await screen.findByText('No connections yet')).toBeTruthy();
	});

	it('is absent for an OAuth provider, whose path is the endpoint form', async () => {
		stubModels({ authType: 'oauth' });
		renderProvider();

		expect(await screen.findByRole('button', { name: 'Add a connection' })).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Add API Key' })).toBeNull();
	});
});

describe('adding one key', () => {
	it('stores it as a connection of this provider and lists it without a reload', async () => {
		const stub = stubModels({ authType: 'api_key' });
		renderProvider();
		await openDialog();

		await fill('Name', 'Primary');
		await fill('API Key', 'sk-aaaaaaaa');
		await fill('Priority', '5');
		await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

		await waitFor(() => expect(stub.endpointCreates).toHaveLength(1));
		// One connection carrying one key: `keys` is what §7.5 reads, and the form's own field names never
		// reach the body.
		expect(stub.endpointCreates[0]).toEqual({
			provider_id: 'openai',
			label: 'Primary',
			auth_type: 'api_key',
			priority: 5,
			keys: [{ value: 'sk-aaaaaaaa' }]
		});
		expect(stub.endpointBulkCreates).toEqual([]);

		expect(await screen.findByText('Primary added.')).toBeTruthy();

		// The page bumped the section's token, so the row the API just stored is in the table. This is the
		// half a dialog-only test cannot hold.
		const table = await screen.findByRole('table', { name: /Upstream endpoints/ });
		expect(within(table).getByText('Primary')).toBeTruthy();
	});

	it("opens clean for the next key, so a second connection cannot inherit the first one's name", async () => {
		const stub = stubModels({ authType: 'api_key' });
		renderProvider();
		await openDialog();

		await fill('Name', 'Primary');
		await fill('API Key', 'sk-aaaaaaaa');
		await fireEvent.click(screen.getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(stub.endpointCreates).toHaveLength(1));

		// The dialog stays mounted between opens, so anything kept is in the next open's field. The browser
		// click-through caught the name surviving a successful add, which is how a second connection ends up
		// carrying the first one's name.
		await openDialog();
		expect(value(screen.getByLabelText('Name'))).toBe('');
		expect(value(screen.getByLabelText('API Key'))).toBe('');
	});
});

describe('adding a pasted list', () => {
	it('stores one connection per line, under the name that line gives it', async () => {
		const stub = stubModels({ authType: 'api_key' });
		renderProvider();
		await openPaste();

		await fill('Keys', 'production|sk-aaaaaaaa\n\nsk-bbbbbbbb');
		await fireEvent.click(screen.getByRole('button', { name: 'Add All Keys' }));

		await waitFor(() => expect(stub.endpointBulkCreates).toHaveLength(1));
		// Two connections, not one holding two keys: the named line keeps the name it was given, and the
		// bare line is named by the planner, which is what the reference's copy calls auto-naming.
		expect(stub.endpointBulkCreates[0]).toEqual({
			provider_id: 'openai',
			auth_type: 'api_key',
			endpoints: [
				{ label: 'production', keys: [{ value: 'sk-aaaaaaaa' }] },
				{ label: 'Key 1', keys: [{ value: 'sk-bbbbbbbb' }] }
			]
		});
		expect(stub.endpointCreates).toEqual([]);

		expect(await screen.findByText('2 added.')).toBeTruthy();

		const table = await screen.findByRole('table', { name: /Upstream endpoints/ });
		expect(within(table).getByText('production')).toBeTruthy();
		expect(within(table).getByText('Key 1')).toBeTruthy();
	});

	it('names a bare line around the names already stored, so a second paste cannot collide', async () => {
		const stub = stubModels({
			authType: 'api_key',
			endpoints: [endpointRow({ id: 'ep_1', label: 'Key 1' })]
		});
		renderProvider();
		await openPaste();

		await fill('Keys', 'sk-cccccccc');
		await fireEvent.click(screen.getByRole('button', { name: 'Add All Keys' }));

		await waitFor(() => expect(stub.endpointBulkCreates).toHaveLength(1));
		// (provider_id, label) is unique, so `Key 1` is taken; the planner gap-fills rather than letting the
		// batch be refused for a name the operator never chose.
		expect(stub.endpointBulkCreates[0].endpoints).toEqual([
			{ label: 'Key 2', keys: [{ value: 'sk-cccccccc' }] }
		]);
	});

	it('reports a refused line by the line it came from, and sends nothing', async () => {
		const stub = stubModels({ authType: 'api_key' });
		renderProvider();
		await openPaste();

		// The blank second line is what makes the third line's number worth asserting: a message keyed to
		// the position among the good rows would say Line 2.
		await fill('Keys', 'sk-aaaaaaaa\n\nsk-b');
		await fireEvent.click(screen.getByRole('button', { name: 'Add All Keys' }));

		expect(await screen.findByText('Line 3: That value looks too short.')).toBeTruthy();
		expect(stub.endpointBulkCreates).toEqual([]);
	});

	it("re-keys the batch's own refusal onto the line it came from", async () => {
		// The label read fails, so the planner cannot see the name that is already stored: the batch is
		// refused by the server, and the server's row index has to land on the pasted line rather than on
		// the batch as a whole.
		const stub = stubModels({
			authType: 'api_key',
			endpointReadStatus: 500,
			endpoints: [endpointRow({ id: 'ep_1', label: 'production' })]
		});
		renderProvider();
		await openPaste();

		await fill('Keys', 'production|sk-aaaaaaaa');
		await fireEvent.click(screen.getByRole('button', { name: 'Add All Keys' }));

		expect(
			await screen.findByText(
				'Line 1: An endpoint with this label already exists for this provider.'
			)
		).toBeTruthy();
		expect(await screen.findByText(/Nothing was added:/)).toBeTruthy();
		expect(stub.endpointBulkCreates).toHaveLength(1);
	});
});
