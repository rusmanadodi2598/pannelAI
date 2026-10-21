// The key control on a provider, and adding one key (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
//
// What this screen owns is the concept: a key added here becomes a connection of this provider, one row per
// key, and the operator is never asked to create a container first. A single key is one `POST /endpoints`.
// The service-account form and the pasted list are `provider-detail-key-modes.test.ts`.

import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { stubModels } from '../support/model-stub';
import { value } from '../support/dom';
import { fill, openDialog, renderProvider } from '../support/provider-key-dialog-harness';

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
describe('the key control on a provider', () => {
	it('is offered where the provider takes a key, under the connections heading', async () => {
		stubModels({ authType: 'api_key' });
		await renderProvider();

		expect(await screen.findByRole('button', { name: 'Add API Key' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'Connections' })).toBeTruthy();
		expect(await screen.findByText('No connections yet')).toBeTruthy();
	});

	it('is absent for an OAuth provider, whose path is the endpoint form', async () => {
		stubModels({ authType: 'oauth' });
		await renderProvider();

		expect(await screen.findByRole('button', { name: 'Add a connection' })).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Add API Key' })).toBeNull();
	});
});

describe('adding one key', () => {
	it('stores it as a connection of this provider and lists it without a reload', async () => {
		const stub = stubModels({ authType: 'api_key' });
		await renderProvider();
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
		await renderProvider();
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
