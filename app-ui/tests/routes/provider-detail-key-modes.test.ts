// The service-account form and the pasted list in the provider key dialog
// (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
//
// Two modes the panel needed to grow: a JSON key file is one credential, not a line to paste, so it gets its
// own form and no paste tab; and a paste is one `POST /endpoints/bulk` carrying one element per line, which
// is the reference's own shape (`providers/[id]/AddApiKeyModal.js`). The single-key path is
// `provider-detail-key-dialog.test.ts`.

import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { endpointRow } from '../support/endpoint-stub';
import { stubModels } from '../support/model-stub';
import { fill, openPaste, renderProvider } from '../support/provider-key-dialog-harness';

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
describe('adding a service-account connection', () => {
	it('asks for the project and location beside the JSON, and stores the pair the route reads', async () => {
		const stub = stubModels({ authType: 'service_account' });
		await renderProvider('vertex');

		await fireEvent.click(await screen.findByRole('button', { name: 'Add Service Account JSON' }));
		await screen.findByLabelText('Name');

		await fill('Name', 'veo');
		await fill('Service Account JSON', '{"type":"service_account"}');
		await fill('Project ID', 'pannelai-prod');
		await fill('Location', 'us-central1');
		await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

		await waitFor(() => expect(stub.endpointCreates).toHaveLength(1));
		// The pair is the route's own field names, and the JSON travels as the connection's key.
		expect(stub.endpointCreates[0]).toEqual({
			provider_id: 'vertex',
			label: 'veo',
			auth_type: 'service_account',
			priority: 1,
			sa_project_id: 'pannelai-prod',
			sa_location: 'us-central1',
			keys: [{ value: '{"type":"service_account"}' }]
		});
	});

	it('refuses the create before the wire when the pair is missing', async () => {
		const stub = stubModels({ authType: 'service_account' });
		await renderProvider('vertex');

		await fireEvent.click(await screen.findByRole('button', { name: 'Add Service Account JSON' }));
		await screen.findByLabelText('Name');

		await fill('Name', 'veo');
		await fill('Service Account JSON', '{"type":"service_account"}');
		await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

		expect(await screen.findByText('A service account endpoint needs a project id.')).toBeTruthy();
		expect(await screen.findByText('A service account endpoint needs a location.')).toBeTruthy();
		expect(stub.endpointCreates).toEqual([]);
	});

	it('offers no paste tab, because a JSON key file is not one credential per line', async () => {
		stubModels({ authType: 'service_account' });
		await renderProvider('vertex');

		await fireEvent.click(await screen.findByRole('button', { name: 'Add Service Account JSON' }));
		await screen.findByLabelText('Name');

		expect(screen.queryByRole('tab', { name: 'Bulk Add' })).toBeNull();
		expect(screen.getByLabelText('Project ID')).toBeTruthy();
	});
});

describe('adding a pasted list', () => {
	it('stores one connection per line, under the name that line gives it', async () => {
		const stub = stubModels({ authType: 'api_key' });
		await renderProvider();
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
		await renderProvider();
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
		await renderProvider();
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
		await renderProvider();
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
